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
import fsnotify "github.com/go-fsnotify/fsnotify"

type executeCB func()
type msgProcessor func(string)

type Listener interface {
	ProcessEvent(string)
}

// Service is a generic goroutine that is both a listener
// and an event creator
type Service interface {
	Listener
	Start()
	Stop()
	AddListener(listener Listener)
}

// DataService also supports retrieval of data and message passing
type DataService interface {
	Service
	GetData() interface{}
	SendMessage(string)
}

type FSWatchService struct {
	watcher     *fsnotify.Watcher
	watches     []string
	listeners   []Listener
	running     bool
	quitChannel chan bool
	flags       fsnotify.Op
	wg          *sync.WaitGroup
}

func (s *FSWatchService) ProcessEvent(string) {
	// no events
	return
}

func (s *FSWatchService) AddListener(listener Listener) {
	s.listeners = append(s.listeners, listener)
}

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
					go s.msgProcessor(val)
				}
			case <-time.After(s.Timeout):
				go s.executor()
			}
		}
	}
}

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

func (s *TimeoutService) SendMessage(msg string) {
	// quit is a reserved message
	if msg != "quit" {
		s.msgChannel <- msg
	}
}

func (s *TimeoutService) AddListener(listener Listener) {
	s.listeners = append(s.listeners, listener)
}

func (s *TimeoutService) ProcessEvent(msg string) {
	s.msgProcessor(msg)
}

type SyncService struct {
	*TimeoutService
}

func (s *SyncService) syncExecuteCB() {
	log.Infof("Execute Database Service Update\n")
	s.mutex.Lock()
	s.libalpm.Mutex.Lock()
	s.libalpm.SyncDBs(false)
	s.libalpm.Mutex.Unlock()
	s.mutex.Unlock()
	go s.notifyListeners("sync_finished")
	log.Infoln("Database update finished")
}

func (s *SyncService) processMsg(msg string) {
	return
}

func (s *SyncService) notifyListeners(msg string) {
	for _, v := range s.listeners {
		v.ProcessEvent(msg)
	}
}

func (s *SyncService) GetData() interface{} {
	return nil
}

type RepoService struct {
	*TimeoutService
	packages            *list.List
	ignoredPackageNames []string
}

func (s *RepoService) repoExecuteCB() {
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

func (s *RepoService) processMsg(msg string) {
	tmsg := strings.Split(msg, ";;")
	switch tmsg[0] {
	case "sync_finished":
		log.Debugln("RepoService: sync_finished event")
		s.repoExecuteCB()
	case "fs_event":
		if len(tmsg) == 3 {
			log.Debugf("RepoService: fs_event: %s %s\n", tmsg[1], tmsg[2])
			if tmsg[2] == "remove" && path.Base(tmsg[1]) == "db.lck" {
				s.repoExecuteCB()
			}
		}
	default:
		return
	}
}

func (s *RepoService) GetData() interface{} {
	var pkgs []*alpm.Pkg
	for e := s.packages.Front(); e != nil; e = e.Next() {
		pkgs = append(pkgs, e.Value.(*alpm.Pkg))
	}
	return pkgs
}

type AURService struct {
	*TimeoutService
	packages *list.List
}

func (s *AURService) aurExecuteCB() {
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

func (s *AURService) GetData() interface{} {
	var pkgs []*alpm.Pkg
	for e := s.packages.Front(); e != nil; e = e.Next() {
		pkgs = append(pkgs, e.Value.(*alpm.Pkg))
	}
	return pkgs
}

func (s *AURService) processMsg(msg string) {
	tmsg := strings.Split(msg, ";;")
	switch tmsg[0] {
	case "sync_finished":
		log.Debugln("RepoService: sync_finished event")
		s.aurExecuteCB()
	case "fs_event":
		if len(tmsg) == 3 {
			log.Debugf("AURService: fs_event: %s %s\n", tmsg[1], tmsg[2])
			if tmsg[2] == "remove" && path.Base(tmsg[1]) == "db.lck" {
				s.aurExecuteCB()
			}
		}
	default:
		return
	}
}

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

func NewRepoService(timeout time.Duration, libalpm *alpm.Alpm,
	conf map[string]map[string]interface{}) *RepoService {
	//base := &Service{msgChannel: make(chan string), running: false}
	tservice := &TimeoutService{Timeout: timeout, libalpm: libalpm, mutex: &sync.Mutex{},
		msgChannel: make(chan string), running: false, conf: conf}
	//tservice := &TimeoutService{base, timeout, libalpm, &sync.Mutex{}, nil, nil, conf}
	service := &RepoService{tservice, list.New(), libalpm.GetIgnoredPackageNames(conf)}
	tservice.setExecuteCB(service.repoExecuteCB)
	tservice.setMsgProcessorCB(service.processMsg)
	return service
}

func NewAURService(timeout time.Duration, libalpm *alpm.Alpm) *AURService {
	//base := &Service{msgChannel: make(chan string), running: false}
	tservice := &TimeoutService{Timeout: timeout, libalpm: libalpm, mutex: &sync.Mutex{},
		msgChannel: make(chan string), running: false}
	//tservice := &TimeoutService{base, timeout, libalpm, &sync.Mutex{}, nil, nil, nil}
	service := &AURService{tservice, list.New()}
	tservice.setExecuteCB(service.aurExecuteCB)
	tservice.setMsgProcessorCB(service.processMsg)
	return service
}

func NewFSWatchService(files []string, flags fsnotify.Op) (*FSWatchService, error) {
	watch, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &FSWatchService{watcher: watch, wg: &sync.WaitGroup{},
		quitChannel: make(chan bool), running: false, watches: files, flags: flags}, nil
}
