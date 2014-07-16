package main

import "pkgupd/alpm"
import "pkgupd/aur"
import "pkgupd/log"
import "sync"
import "time"
import "container/list"

type executeCB func()
type msgProcessor func(string)

type Listener interface {
	ProcessEvent(string)
}

type ServiceRunner interface {
	Listener
	Start()
	Stop()
	GetData() interface{}
	SendMessage(string)
	AddListener(listener Listener)
}

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

type SyncService struct {
	*TimeoutService
}

type RepoService struct {
	*TimeoutService
	packages            *list.List
	ignoredPackageNames []string
}

type AURService struct {
	*TimeoutService
	packages *list.List
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
					s.msgProcessor(val)
				}
			case <-time.After(s.Timeout):
				s.executor()
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
	switch msg {
	case "sync_finished":
		s.repoExecuteCB()
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
	switch msg {
	case "sync_finished":
		s.aurExecuteCB()
	default:
		return
	}
}

func NewSyncService(timeout time.Duration, libalpm *alpm.Alpm) *SyncService {
	tservice := &TimeoutService{Timeout: timeout, libalpm: libalpm, mutex: &sync.Mutex{},
		msgChannel: make(chan string), running: false}
	service := &SyncService{tservice}
	tservice.setExecuteCB(service.syncExecuteCB)
	tservice.setMsgProcessorCB(service.processMsg)
	return service
}

func NewRepoService(timeout time.Duration, libalpm *alpm.Alpm,
	conf map[string]map[string]interface{}) *RepoService {
	tservice := &TimeoutService{Timeout: timeout, libalpm: libalpm, mutex: &sync.Mutex{},
		msgChannel: make(chan string), running: false, conf: conf}
	service := &RepoService{tservice, list.New(), libalpm.GetIgnoredPackageNames(conf)}
	tservice.setExecuteCB(service.repoExecuteCB)
	tservice.setMsgProcessorCB(service.processMsg)
	return service
}

func NewAURService(timeout time.Duration, libalpm *alpm.Alpm) *AURService {
	tservice := &TimeoutService{Timeout: timeout, libalpm: libalpm, mutex: &sync.Mutex{},
		msgChannel: make(chan string), running: false}
	service := &AURService{tservice, list.New()}
	tservice.setExecuteCB(service.aurExecuteCB)
	tservice.setMsgProcessorCB(service.processMsg)
	return service
}
