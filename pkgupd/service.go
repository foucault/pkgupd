package main

import "pkgupd/alpm"
import "pkgupd/aur"
import "pkgupd/log"
import "sync"
import "time"
import "fmt"
import "container/list"
import "errors"
import "strings"
import "path"
import fsnotify "github.com/fsnotify/fsnotify"

type executeCB func(args ...string)
type msgProcessor func(string)

// Listener is an interface that should be implemented
// by all types that expect to read events from services.
// Events are string messages with fields seperated by
// double semicolons.
type Listener interface {
	// The event that has been received from the service
	ProcessEvent(string)
}

// Service is a generic goroutine that is both a listener
// and an event creator. All services should expect events
// from other listeners so they must implement the Listener
// interface. Services also accept other listeners that can
// be notified upon request
type Service interface {
	Listener
	// Start starts the service (usually as goroutine)
	Start()
	// Stop stops the services
	Stop()
	// AddListener subscribes a type to receive events from
	// this service
	AddListener(listener Listener)
}

// DataService is a service that also supports retrieval of
// data and message passing
type DataService interface {
	Service
	GetData() interface{}
	SendMessage(string)
}

// FSWatchService implements the Service interface and
// provides events to listeners for filesystem changes.
// The event pattern is fs_notify;;FILENAME;;EVT_TYPE
type FSWatchService struct {
	watcher     *fsnotify.Watcher
	watches     []string
	listeners   []Listener
	running     bool
	quitChannel chan bool
	flags       fsnotify.Op
	wg          *sync.WaitGroup
}

// ProcessEvent is only implemented to fulfill the Listener
// interface but no events are required to be processed by
// this service.
func (s *FSWatchService) ProcessEvent(string) {
	// no events
	return
}

// AddListener implements the AddListener method for interface
// Listener
func (s *FSWatchService) AddListener(listener Listener) {
	s.listeners = append(s.listeners, listener)
}

// Start starts the watch service
func (s *FSWatchService) Start() {
	if !s.running {
		s.running = true
		s.wg.Add(1)
		for _, v := range s.watches {
			s.watcher.Add(v)
		}
		log.Debugln("Filesystem watch started")
	serviceLoop:
		for {
			select {
			case event := <-s.watcher.Events:
				ret, err := s.eventType(&event)
				if err == nil {
					log.Debugln(ret)
					s.notifyListeners(ret)
				}
			case err := <-s.watcher.Errors:
				log.Errorln(err)
			case <-s.quitChannel:
				log.Debugln("Stopping filesystem watch")
				for _, watch := range s.watches {
					s.watcher.Remove(watch)
				}
				break serviceLoop
			}
		}
		s.watcher.Close()
		s.wg.Done()
		log.Debugln("Filesystem watch stopped")
	}
}

// Stop stops the watch service. This blocks until all events
// are flushed
func (s *FSWatchService) Stop() {
	if s.running {
		s.quitChannel <- true
		s.wg.Wait()
		s.running = false
	}
}

func (s *FSWatchService) notifyListeners(msg string) {
	for _, l := range s.listeners {
		l.ProcessEvent("fs_event;;" + msg)
	}
}

func (s *FSWatchService) eventType(evt *fsnotify.Event) (string, error) {
	evtName := evt.Name
	switch evt.Op & s.flags {
	case fsnotify.Create:
		return fmt.Sprintf("%s;;%s", evtName, "create"), nil
	case fsnotify.Remove:
		return fmt.Sprintf("%s;;%s", evtName, "remove"), nil
	case fsnotify.Write:
		return fmt.Sprintf("%s;;%s", evtName, "write"), nil
	case fsnotify.Chmod:
		return fmt.Sprintf("%s;;%s", evtName, "chmod"), nil
	case fsnotify.Rename:
		return fmt.Sprintf("%s;;%s", evtName, "rename"), nil
	}
	return "", errors.New("unknown event type")
}

// TimeoutService is a service that executes alpm
// processes on a given interval or at request
// Implements DataService
type TimeoutService struct {
	Timeout      time.Duration
	libalpm      *alpm.Alpm
	mutex        *sync.Mutex
	msgChannel   chan string
	running      bool
	executor     executeCB
	msgProcessor msgProcessor
	listeners    []Listener
	conf         map[string]map[string]interface{}
}

// Start starts the timeout service
func (s *TimeoutService) Start() {
	if !s.running {
		s.running = true
		s.executor()
	serviceLoop:
		for {
			select {
			case val := <-s.msgChannel:
				if val == "quit" {
					s.running = false
					break serviceLoop
				} else {
					s.msgProcessor(val)
				}
			case <-time.After(s.Timeout):
				s.executor()
			}
		}
	}
}

// Stop stops the timeout service
func (s *TimeoutService) Stop() {
	if s.running {
		s.msgChannel <- "quit"
	}
	s.running = false
}

func (s *TimeoutService) setExecuteCB(cb executeCB) {
	s.executor = cb
}

func (s *TimeoutService) setMsgProcessorCB(cb msgProcessor) {
	s.msgProcessor = cb
}

// SendMessage sends a message to the timeout service that is
// processed from the msgProcessor callback. Message "quit" is
// reserved and always break the service main loop so it cannot
// be passed to the service. If you want to stop the service
// use TimeoutService.Stop()
func (s *TimeoutService) SendMessage(msg string) {
	// quit is a reserved message
	if msg != "quit" {
		s.msgChannel <- msg
	}
}

// AddListener adds a new listener to be notified for
// events from this service
func (s *TimeoutService) AddListener(listener Listener) {
	s.listeners = append(s.listeners, listener)
}

// ProcessEvent calls the message processor callback on the
// specified message
func (s *TimeoutService) ProcessEvent(msg string) {
	s.msgProcessor(msg)
}

// SyncService is a timeout service that syncs
// pacman databases
type SyncService struct {
	*TimeoutService
}

// The executor callback
func (s *SyncService) syncExecuteCB(args ...string) {

	force := stringInList(args, "force")

	log.Infof("Execute Database Service Update\n")
	s.mutex.Lock()
	s.libalpm.Mutex.Lock()
	didSync := s.libalpm.SyncDBs(force)
	s.libalpm.Mutex.Unlock()
	s.mutex.Unlock()
	if didSync {
		log.Debugln("Databases changed, notifying listeners")
		s.notifyListeners("sync_finished")
	} else {
		log.Debugln("Database not changed, no need to notify listeners")
	}
	log.Infoln("Database update finished")
}

// The message processor callback
func (s *SyncService) processMsg(msg string) {
	tmsg := strings.Split(msg, ";;")
	switch tmsg[0] {
	case "force_sync":
		s.syncExecuteCB("force")
	default:
		return
	}
}

func (s *SyncService) notifyListeners(msg string) {
	for _, v := range s.listeners {
		v.ProcessEvent(msg)
	}
}

// GetData always returns nil for SyncService
func (s *SyncService) GetData() interface{} {
	return nil
}

// RepoService is a timeout service that retrieves
// local package updates from the pacman database
type RepoService struct {
	*TimeoutService
	packages            *list.List
	ignoredPackageNames []string
	dbChanged           bool
}

// The executor callback
func (s *RepoService) repoExecuteCB(args ...string) {
	log.Infoln("Execute Repo Service Update")
	s.mutex.Lock()
	s.packages = s.packages.Init()
	updPkgs := s.libalpm.GetUpdates()
	for _, v := range updPkgs {
		if stringInList(s.ignoredPackageNames, v.Name) {
			continue
		}
		s.packages.PushBack(v)
	}
	s.mutex.Unlock()
	log.Infoln("Repo update finished")
}

// The message processor callback
func (s *RepoService) processMsg(msg string) {
	tmsg := strings.Split(msg, ";;")
	switch tmsg[0] {
	case "sync_finished":
		log.Debugln("RepoService: sync_finished event")
		s.repoExecuteCB()
	case "fs_event":
		if len(tmsg) == 3 {
			log.Debugf("RepoService: fs_event: %s %s\n", tmsg[1], tmsg[2])
			if path.Base(tmsg[1]) != "db.lck" {
				s.dbChanged = true
			} else if tmsg[2] == "remove" && path.Base(tmsg[1]) == "db.lck" {
				if s.dbChanged {
					log.Debugln("RepoService: Database changed and lock removed, updating")
					s.repoExecuteCB()
					s.dbChanged = false
				} else {
					log.Debugln("RepoService: Database lock detected but no changes made")
				}
			}
		}
	default:
		return
	}
}

// GetData returns the local package updates and its
// type is []*alpm.Pkg
func (s *RepoService) GetData() interface{} {
	var pkgs []*alpm.Pkg
	for e := s.packages.Front(); e != nil; e = e.Next() {
		pkgs = append(pkgs, e.Value.(*alpm.Pkg))
	}
	return pkgs
}

// AURService is a timeout services that retrieves the
// remote version of foreign packages and checks for updates
type AURService struct {
	*TimeoutService
	packages  *list.List
	dbChanged bool
}

// The executor callback
func (s *AURService) aurExecuteCB(args ...string) {
	log.Infof("Execute AUR Service Update\n")
	s.mutex.Lock()
	s.packages = s.packages.Init()
	fpkgs := s.libalpm.GetForeign()
	aur.UpdateRemoteVersions(fpkgs)
	for _, v := range fpkgs {
		if v.RemoteVersion == "0" {
			continue
		}
		if v.IsUpdatable() {
			s.packages.PushBack(v)
		}
	}
	s.mutex.Unlock()
	log.Infoln("AUR update finished")
}

// GetData returns a slice of AUR-updatable foreign packages.
// The return type is []*alpm.Pkg
func (s *AURService) GetData() interface{} {
	var pkgs []*alpm.Pkg
	for e := s.packages.Front(); e != nil; e = e.Next() {
		pkgs = append(pkgs, e.Value.(*alpm.Pkg))
	}
	return pkgs
}

// The message processor callback
func (s *AURService) processMsg(msg string) {
	tmsg := strings.Split(msg, ";;")
	switch tmsg[0] {
	case "sync_finished":
		log.Debugln("RepoService: sync_finished event")
		s.aurExecuteCB()
	case "fs_event":
		if len(tmsg) == 3 {
			log.Debugf("AURService: fs_event: %s %s\n", tmsg[1], tmsg[2])
			if path.Base(tmsg[1]) != "db.lck" {
				s.dbChanged = true
			} else if tmsg[2] == "remove" && path.Base(tmsg[1]) == "db.lck" {
				if s.dbChanged {
					log.Debugln("AURService: Database changed and lock removed, updating")
					s.aurExecuteCB()
					s.dbChanged = false
				} else {
					log.Debugln("AURService: Database lock detected but no changes made")
				}
			}
		}
	default:
		return
	}
}

// NewSyncService creates a new sync service. It requires the timeout
// interval and a pointer to an initialized libalpm.
func NewSyncService(timeout time.Duration, libalpm *alpm.Alpm) *SyncService {
	//base := &Service{msgChannel: make(chan string), running: false}
	tservice := &TimeoutService{Timeout: timeout, libalpm: libalpm, mutex: &sync.Mutex{},
		msgChannel: make(chan string), running: false}
	//tservice := &TimeoutService{base, timeout, libalpm, &sync.Mutex{}, nil, nil, nil}
	service := &SyncService{tservice}
	tservice.setExecuteCB(service.syncExecuteCB)
	tservice.setMsgProcessorCB(service.processMsg)
	return service
}

// NewRepoService creates a new repo service. It requires the timeout
// interval, a pointer to an initialized libalpm and the pacman.conf
// configuration map.
func NewRepoService(timeout time.Duration, libalpm *alpm.Alpm,
	conf map[string]map[string]interface{}) *RepoService {
	tservice := &TimeoutService{Timeout: timeout, libalpm: libalpm, mutex: &sync.Mutex{},
		msgChannel: make(chan string), running: false, conf: conf}
	service := &RepoService{tservice, list.New(), libalpm.GetIgnoredPackageNames(conf), false}
	tservice.setExecuteCB(service.repoExecuteCB)
	tservice.setMsgProcessorCB(service.processMsg)
	return service
}

// NewAURService creates a new AUR service. It requires the timeout
// interval and a pointer to an initialized libalpm.
func NewAURService(timeout time.Duration, libalpm *alpm.Alpm) *AURService {
	tservice := &TimeoutService{Timeout: timeout, libalpm: libalpm, mutex: &sync.Mutex{},
		msgChannel: make(chan string), running: false}
	service := &AURService{tservice, list.New(), false}
	tservice.setExecuteCB(service.aurExecuteCB)
	tservice.setMsgProcessorCB(service.processMsg)
	return service
}

// NewFSWatchService creates a new filesystem watch service. It requires
// a list of watched files or folders and a flag mask of events to
// monitor, for example fsnotify.Create|fsnotify.Remove will only send
// notifications for create and remove events.
func NewFSWatchService(files []string, flags fsnotify.Op) (*FSWatchService, error) {
	watch, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &FSWatchService{watcher: watch, wg: &sync.WaitGroup{},
		quitChannel: make(chan bool), running: false, watches: files, flags: flags}, nil
}
