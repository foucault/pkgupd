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
import "errors"
import "strings"
import "fmt"

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

type Alpm struct {
	RootPath string
	LibPath  string
	Mutex    *sync.Mutex
	dbs      *C.alpm_list_t
}

type Pkg struct {
	Name          string
	LocalVersion  string
	RemoteVersion string
	Foreign       bool
}

func (p *Pkg) IsUpdatable() bool {
	ret := VerCmp(p.LocalVersion, p.RemoteVersion)
	if ret < 0 {
		return true
	}
	return false
}

// This returns a map with string keys and map values. The maps
// are map[string]interface{}. inteface{} is either string or a []string
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
					serverUrl := strings.Trim(serverEntry[1], " \t\r\n")
					serverUrl = strings.Replace(serverUrl, "$repo", repo, -1)
					serverUrl = strings.Replace(serverUrl, "$arch", arch, -1)
					ires = append(ires, serverUrl)
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

func NewAlpm(root string, lib string) (*Alpm, error) {
	if _, err := os.Stat(lib); os.IsNotExist(err) {
		return nil, errors.New(fmt.Sprintf("Library path '%s' does not exit", lib))
	}
	ret := &Alpm{RootPath: root, LibPath: lib, Mutex: &sync.Mutex{}}
	C.init_paths(C.CString(root), C.CString(lib))
	return ret, nil
}

func (a *Alpm) AddDatabase(name string, servers []string) error {
	db := C.new_syncdb(C.CString(name))
	for _, v := range servers {
		C.add_server_to_syncdb(db, C.CString(v))
	}
	a.dbs = C.alpm_list_add(a.dbs, unsafe.Pointer(db))
	return nil
}

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

func (a *Alpm) SyncDBs(force bool) {
	_force := 0
	if force {
		_force = 1
	}
	C.sync_dbs(a.dbs, C.int(_force))
}

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

func (a *Alpm) Close() {
	C.free_syncdb_list(a.dbs)
	C.goalpm_cleanup()
}

func freeStr(ptr *C.char) {
	C.free(unsafe.Pointer(ptr))
}

func PkgVer(pkgname string) string {
	cpkgname := C.CString(pkgname)
	defer freeStr(cpkgname)
	return C.GoString(C.pkgver(cpkgname))
}
