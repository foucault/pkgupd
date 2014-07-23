// Package alpm provides a cgo wrapper around alpm for functions used
// by the pkgupd server
package alpm

/*
#cgo LDFLAGS: -L. -lalpm -lgoalpm
#include <alpm.h>
#include "goalpm.h"
*/
import "C"

import "unsafe"
import "regexp"
import "os"
import "bufio"
import "sync"
import "container/list"
import "strings"
import "fmt"

// Remove duplicate strings from string list
func deduplicateStringList(a []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, v := range a {
		if _, ok := seen[v]; !ok {
			result = append(result, v)
			seen[v] = true
		}
	}
	return result
}

// Alpm represents a libalpm instance
type Alpm struct {
	// The root path of the filesystem upon which
	// this libalpm will operate (usually "/")
	RootPath string
	// The path of the local/sync library used by libalpm
	LibPath string
	// A mutex used for operations that require write access
	// to the database (these create a db.lck file in the base
	// path of the database). The mutex must be acquired if
	// a write operation is needed.
	Mutex *sync.Mutex
	// A list of sync dbs
	dbs *C.alpm_list_t
}

// Pkg represents a pacman package
type Pkg struct {
	// Name of the package
	Name string
	// Local version of the package
	LocalVersion string
	// Remote version of the package. If there is no remote
	// package available this should be 0
	RemoteVersion string
	// This is true if the package has no entry in the local
	// database or on any remote sync database
	Foreign bool
}

// IsUpdatable checks if this package is updatable. If
// remote version is zero this is always false.
func (p *Pkg) IsUpdatable() bool {
	return IsUpdatablePkg(p)
}

// ParsePacmanConf reads the specified pacman configuration and returns a map
// with string keys and map values. The maps are map[string]interface{}.
// interface{} is either string or a []string
func ParsePacmanConf(conf string) (map[string]map[string]interface{}, error) {
	// matches "[foo]" and puts "foo" in the first match group
	sectionRE := regexp.MustCompile(`\[(.*)\]`)
	// matches "key = val" or just "key"; puts "key" in the first match group
	// and "val" (if it exists) in second match group
	stateRE := regexp.MustCompile(`(\w*)\s*=?\s*(.*)`)
	file, err := os.Open(conf)
	if err != nil {
		return nil, err
	}
	currentSection := "options"
	var line string
	confMap := make(map[string]map[string]interface{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line = scanner.Text()
		line = strings.Trim(line, " \t\r\n")
		if strings.HasPrefix(line, "#") || len(line) == 0 {
			continue
		}
		sections := sectionRE.FindStringSubmatch(line)
		if len(sections) > 0 {
			currentSection = sections[1]
			if _, ok := confMap[currentSection]; !ok {
				confMap[currentSection] = make(map[string]interface{})
			}
			continue
		}
		states := stateRE.FindStringSubmatch(line)
		key := states[1]
		val := states[2]
		if len(states) > 0 {
			if key == "Include" || key == "Server" {
				if _, ok := confMap[currentSection][key]; !ok {
					confMap[currentSection][key] = []string{val}
				} else {
					confMap[currentSection][key] =
						append(confMap[currentSection][key].([]string), val)
				}
			} else {
				confMap[currentSection][key] = val
			}
		}
	}
	file.Close()
	return confMap, nil
}

// GetServersFromConf searches all Include and Server directives
// found in repo section in pacman.conf and returns a list of all servers
// that correspond to this repository. Duplicate servers are discarded.
// The required arguments is a map of the values under the repo section (conf),
// the name of the repo (repo) and the architecture of the machine (arch).
// BUG: This always returns the Included servers first and the the single
// Server directives in the section, even if they are specified in a different
// order
func GetServersFromConf(conf map[string]interface{}, repo string, arch string) []string {
	var ires []string
	if _, ok := conf["Include"]; ok {
		var scanner *bufio.Scanner
		var line string
		var serverEntry []string
		for _, entry := range conf["Include"].([]string) {
			file, err := os.Open(entry)
			if err != nil {
				continue
			}
			scanner = bufio.NewScanner(file)
			for scanner.Scan() {
				line = strings.Trim(scanner.Text(), " \t\r\n")
				if strings.HasPrefix(line, "#") || len(line) == 0 {
					continue
				}
				serverEntry = strings.Split(line, "=")
				if len(serverEntry) == 2 {
					serverURL := strings.Trim(serverEntry[1], " \t\r\n")
					serverURL = strings.Replace(serverURL, "$repo", repo, -1)
					serverURL = strings.Replace(serverURL, "$arch", arch, -1)
					ires = append(ires, serverURL)
				}
			}
			file.Close()
		}
	}
	if _, ok := conf["Server"]; ok {
		for _, entry := range conf["Server"].([]string) {
			ires = append(ires, entry)
		}
	}
	return deduplicateStringList(ires)
}

// NewAlpm returns a new instance of the libalpm library. It requires a
// path to the root of the pacman operations and a path to the root of
// the pacman local/sync library.
func NewAlpm(root string, lib string) (*Alpm, error) {
	if _, err := os.Stat(lib); os.IsNotExist(err) {
		return nil, fmt.Errorf("Library path '%s' does not exit", lib)
	}
	ret := &Alpm{RootPath: root, LibPath: lib, Mutex: &sync.Mutex{}}
	C.init_paths(C.CString(root), C.CString(lib))
	return ret, nil
}

// AddDatabase adds a new database named "name" with a list of
// sync servers (servers) to the libalpm.
func (a *Alpm) AddDatabase(name string, servers []string) error {
	db := C.new_syncdb(C.CString(name))
	for _, v := range servers {
		C.add_server_to_syncdb(db, C.CString(v))
	}
	a.dbs = C.alpm_list_add(a.dbs, unsafe.Pointer(db))
	return nil
}

// GetUpdates returns a slice of package ([]*Pkg) that are updatable.
// Only local packages with remote versions in a repo are included.
func (a *Alpm) GetUpdates() []*Pkg {
	var pkglist []*Pkg

	var upkg *C.upd_package
	res := C.get_updates(a.dbs)
	for it := res; it != nil; it = C.alpm_list_next(it) {
		upkg = (*C.upd_package)(it.data)
		pkglist = append(pkglist,
			&Pkg{C.GoString(upkg.name),
				C.GoString(upkg.loc_version),
				C.GoString(upkg.rem_version), false})
	}
	C.free_pkg_list(res)
	return pkglist
}

// GetUpdatesList is the same as GetUpdates but returns a container/list.List
// instead of a slice.
func (a *Alpm) GetUpdatesList() *list.List {
	pkglist := list.New()

	res := C.get_updates(a.dbs)
	var upkg *C.upd_package
	for it := res; it != nil; it = C.alpm_list_next(it) {
		upkg = (*C.upd_package)(it.data)
		pkglist.PushBack(&Pkg{C.GoString(upkg.name),
			C.GoString(upkg.loc_version),
			C.GoString(upkg.rem_version), false})
	}
	C.free_pkg_list(res)
	return pkglist

}

// GetForeign returns a slice of all foreign packages ([]*Pkg)
func (a *Alpm) GetForeign() []*Pkg {
	var pkglist []*Pkg

	res := C.get_foreign(a.dbs)
	var upkg *C.upd_package
	for it := res; it != nil; it = C.alpm_list_next(it) {
		upkg = (*C.upd_package)(it.data)
		pkglist = append(pkglist,
			&Pkg{C.GoString(upkg.name),
				C.GoString(upkg.loc_version),
				C.GoString(upkg.rem_version), true})
	}
	C.free_pkg_list(res)
	return pkglist
}

// GetForeignList is the same as GetForeign but returns a container/list.List
// instead of a slice.
func (a *Alpm) GetForeignList() *list.List {
	pkglist := list.New()

	res := C.get_foreign(a.dbs)
	var upkg *C.upd_package
	for it := res; it != nil; it = C.alpm_list_next(it) {
		upkg = (*C.upd_package)(it.data)
		pkglist.PushBack(&Pkg{C.GoString(upkg.name),
			C.GoString(upkg.loc_version),
			C.GoString(upkg.rem_version), false})
	}
	C.free_pkg_list(res)
	return pkglist
}

// SyncDBs synchronizes the databases. Set force to true to redownload
// the databases even if they are up-to-date.
func (a *Alpm) SyncDBs(force bool) {
	_force := 0
	if force {
		_force = 1
	}
	C.sync_dbs(a.dbs, C.int(_force))
}

// GetGroupPackageNames returns a slice of string including all the
// package names that fall under the specified group.
func (a *Alpm) GetGroupPackageNames(group string) []string {
	ret := []string{}
	cgroup := C.CString(group)
	defer freeStr(cgroup)
	res := C.get_group_pkgs(cgroup)

	for it := res; it != nil; it = C.alpm_list_next(it) {
		ret = append(ret, C.GoString((*C.char)(it.data)))
	}
	return ret
}

// GetIgnoredPackageNames returns a list of package names that are
// ignored based on the parsed pacman.conf configuration. This includes
// both IngorePkg and IgnoreGroup. IgnoreGroup packages are expanded to
// the included packages.
func (a *Alpm) GetIgnoredPackageNames(conf map[string]map[string]interface{}) []string {
	ignoredPkgs := []string{}
	if val, ok := conf["options"]["IgnorePkg"].(string); ok {
		for _, v := range strings.Split(val, " ") {
			ignoredPkgs = append(ignoredPkgs, strings.Trim(v, " \t\r\n"))
		}
	}

	if val, ok := conf["options"]["IgnoreGroup"].(string); ok {
		var grp string
		var grpPkgs []string
		for _, v := range strings.Split(val, " ") {
			grp = strings.Trim(v, " \t\r\n")
			fmt.Printf("Ignoring group '%s' packages\n", grp)
			grpPkgs = a.GetGroupPackageNames(grp)
			fmt.Println(grpPkgs)
			ignoredPkgs = append(ignoredPkgs, grpPkgs...)
		}
	}
	return ignoredPkgs
}

// Close deinitializes libalpm and frees allocated resources
func (a *Alpm) Close() {
	C.free_syncdb_list(a.dbs)
	C.goalpm_cleanup()
}

func freeStr(ptr *C.char) {
	C.free(unsafe.Pointer(ptr))
}

// PkgVer returns the version of the package named pkgname
func PkgVer(pkgname string) string {
	cpkgname := C.CString(pkgname)
	defer freeStr(cpkgname)
	return C.GoString(C.pkgver(cpkgname))
}
