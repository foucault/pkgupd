// Package main is the main executable for the pkgupd server
package main

import "pkgupd/alpm"
import "fmt"
import "os"
import "os/signal"
import "syscall"
import "time"
import "runtime"
import "runtime/pprof"
import "runtime/debug"
import "github.com/jessevdk/go-flags"
import "path"
import "strings"
import "errors"
import "io"

import "pkgupd/log"

// Path of the pacman database
const PacmanDB = "/var/lib/pacman"

// Error codes for SandboxError
const (
	_ = iota
	MissingSandboxDir
	MissingLocalDir
	MissingSyncDir
)

// DBPathError is an error type for missing databases
type DBPathError struct {
	// A list of missing sync databases
	MissingDBs []string
}

// SandboxError is an error type for errors occuring
// during sandbox fsck.
type SandboxError struct {
	// The code of the error
	ErrorCode int
}

func (d *DBPathError) Error() string {
	return "Missing dbs: " + strings.Join(d.MissingDBs, ", ")
}

func (s *SandboxError) Error() string {
	switch s.ErrorCode {
	case MissingSandboxDir:
		return "Sandbox directory does not exist"
	case MissingLocalDir:
		return "Local db directory does not exist or is not a directory"
	case MissingSyncDir:
		return "Sync db directory does not exist"
	default:
		return "Unspecified sandbox error"
	}
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	} else if _, ok := err.(*os.PathError); ok && os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func pathIsDirectory(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err == nil {
		//File exists
		if fi.IsDir() {
			return true, nil
		}
		return false, nil
	} else if _, ok := err.(*os.PathError); ok && os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func pathIsSymlink(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err == nil {
		//File exists and is link
		if (fi.Mode())&os.ModeSymlink != 0 {
			return true, nil
		}
		return false, nil
	} else if _, ok := err.(*os.PathError); ok && os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func pathIsSymlinkDir(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err == nil {
		//File exists and is link
		mode := fi.Mode()
		if (mode&os.ModeSymlink != 0) && (mode&os.ModeDir != 0) {
			return true, nil
		}
		return false, nil
	} else if _, ok := err.(*os.PathError); ok && os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func copyFile(src string, dst string) error {
	s, err := os.Open(src)
	defer s.Close()
	if err != nil {
		return err
	}
	sinfo, err := s.Stat()
	if err != nil {
		return err
	}
	modtime := sinfo.ModTime()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(d, s); err != nil {
		d.Close()
		return err
	}
	// copy mod time over
	err = os.Chtimes(dst, modtime, modtime)
	if err != nil {
		d.Close()
		return err
	}
	return d.Close()
}

func fsckSandbox(dbpath string, conf map[string]map[string]interface{}) error {

	isSandboxDir, _ := pathIsDirectory(dbpath)
	if !isSandboxDir {
		return &SandboxError{MissingSandboxDir}
	}
	isSyncDir, _ := pathIsDirectory(path.Join(dbpath, "sync"))
	if !isSyncDir {
		return &SandboxError{MissingSyncDir}
	}
	isLocalDirSymlink, _ := pathIsSymlinkDir(path.Join(dbpath, "local"))
	isLocalDir, _ := pathIsDirectory(path.Join(dbpath, "local"))
	if !isLocalDir && !isLocalDirSymlink {
		return &SandboxError{MissingLocalDir}
	}

	var missingDBs []string
	var dbExists bool

	for k := range conf {
		if k == "options" {
			continue
		}
		dbExists, _ = pathExists(path.Join(dbpath, "sync", k+".db"))
		if !dbExists {
			missingDBs = append(missingDBs, k)
		}
	}
	if len(missingDBs) != 0 {
		return &DBPathError{missingDBs}
	}

	return nil
}

func fixSandbox(dbpath string, conf map[string]map[string]interface{}) error {
	iterations := 0
	var err error
	for {
		err = fsckSandbox(dbpath, conf)
		if e, ok := err.(*SandboxError); ok {
			switch e.ErrorCode {
			case MissingSandboxDir:
				er := os.Mkdir(dbpath, 0755)
				if er != nil {
					return errors.New("Could not create sandbox dir " + er.Error())
				}
				iterations--
			case MissingSyncDir:
				er := os.Mkdir(path.Join(dbpath, "sync"), 0755)
				if er != nil {
					return errors.New("Could not create sync dir " + er.Error())
				}
				iterations--
			case MissingLocalDir:
				er := os.Symlink(path.Join(PacmanDB, "local"), path.Join(dbpath, "local"))
				if er != nil {
					return errors.New("Could not symlink local db " + er.Error())
				}
				iterations--
			}
		} else if er, ok := err.(*DBPathError); ok {
			log.Infoln(er)
			for _, db := range er.MissingDBs {
				cer := copyFile(path.Join(PacmanDB, "sync", db+".db"),
					path.Join(dbpath, "sync", db+".db"))
				if cer != nil {
					fmt.Println("Could not copy database", db, ".", cer)
				}
			}
			return nil
		} else {
			return errors.New("Error while checking sandbox: " + err.Error())
		}
		iterations++
		if iterations >= 10 {
			return errors.New("Too many fsck iterations")
		}
	}
	return nil
}

func goRuntimeStats() {
	m := &runtime.MemStats{}
	log.Debugln("# goroutines: ", runtime.NumGoroutine())
	runtime.ReadMemStats(m)
	log.Debugln("Memory Acquired: ", m.Sys)
	log.Debugln("Memory Used    : ", m.Alloc)
}

func main() {

	var opts Options
	parser := flags.NewParser(&opts, flags.Default)
	if _, err := parser.Parse(); err != nil {
		if _, ok := err.(*flags.Error); ok {
			if serr, _ := err.(*flags.Error); serr.Type == flags.ErrHelp {
				os.Exit(0)
			} else {
				fmt.Println("Error parsing command line:", err)
			}
		}
		os.Exit(1)
	}

	switch len(opts.Verbose) {
	case 0:
		log.SetLevel(log.LogWarn)
	case 1:
		log.SetLevel(log.LogInfo)
	default:
		log.SetLevel(log.LogDebug)
	}
	// Parse configuration
	conf, err := alpm.ParsePacmanConf(string(opts.PacmanConf))
	if err != nil {
		log.ErrorFatal("Could not parse configuration:", err)
	}

	// Extract system architecture
	arch := systemArch()
	if val, ok := conf["options"]["Architecture"].(string); ok {
		if val != "auto" {
			arch = val
		}
	}

	err = fsckSandbox(string(opts.DBRoot), conf)
	if err != nil {
		fixerr := fixSandbox(string(opts.DBRoot), conf)
		if fixerr != nil {
			log.Errorln("Sandbox fsck failed, bailing out", fixerr)
			os.Exit(1)
		} else {
			log.Warn("Sandbox fsck failed, but errors fixed")
		}
	}

	libalpm, err := alpm.NewAlpm("/", string(opts.DBRoot))
	if err != nil {
		log.Errorln("Could not initialize libalpm:", err)
		os.Exit(1)
	}
	for k, v := range conf {
		if k == "options" {
			continue
		}
		servers := alpm.GetServersFromConf(v, k, arch)
		libalpm.AddDatabase(k, servers)
	}

	server := NewServer(opts.NotifyFS)
	services := make(map[string]DataService)
	services["repo"] = NewRepoService(time.Duration((opts.PollInterval))*time.Second, libalpm, conf)
	if opts.EnableAUR {
		log.Infoln("Enabling AUR Service")
		services["aur"] = NewAURService(time.Duration(opts.AURInterval)*time.Second, libalpm)
	}
	if opts.EnableSync {
		log.Infoln("Enabling Sync Service")
		syncService := NewSyncService(time.Duration(opts.SyncInterval)*time.Second, libalpm)
		for _, v := range services {
			syncService.AddListener(v)
		}
		server.AddService("sync", syncService)
	}

	for k, v := range services {
		server.AddService(k, v)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)

	go server.Serve(opts.ListenType, opts.ListenAddr)
	server.Start()

mainloop:
	for {
		select {
		case sig := <-sigs:
			log.Infoln("Received", sig, "signal")
			if sig == syscall.SIGTERM || sig == syscall.SIGINT {
				break mainloop
			}
			if sig == syscall.SIGUSR1 {
				f, err := os.Create("/tmp/pkgupd.prof")
				if err == nil {
					pprof.WriteHeapProfile(f)
				}
				f.Close()
				log.Debugln("Mem stats before FreeOSMemory")
				goRuntimeStats()
				debug.FreeOSMemory()
				log.Debugln("Mem stats after FreeOSMemory")
				goRuntimeStats()

			}
		case err := <-server.serverError:
			if err == true {
				log.Errorln("Server error")
				break mainloop
			}
		}
	}

	server.Stop()
	log.Infoln("Waiting for server to stop")
	server.Wait()
	libalpm.Close()
	log.Infoln("Exiting")
}
