package main

import "pkgupd/alpm"
import "pkgupd/aur"
import "runtime"
import "errors"
import "fmt"
import "strings"

func testRun(libalpm *alpm.Alpm, conf map[string]map[string]interface{}) {
	fmt.Printf("Syncing databases.... ")
	// Sync the databases
	libalpm.SyncDBs(false)
	fmt.Println("Done!")

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
			grpPkgs = libalpm.GetGroupPackageNames(grp)
			fmt.Println(grpPkgs)
			ignoredPkgs = append(ignoredPkgs, grpPkgs...)
		}
	}

	// Print local updates
	for _, p := range libalpm.GetUpdates() {
		if stringInList(ignoredPkgs, p.Name) {
			fmt.Printf("[LOCAL] %s is updatable but ignored\n", p.Name)
		} else {
			fmt.Printf("[LOCAL] %s %s -> %s\n", p.Name, p.LocalVersion, p.RemoteVersion)
		}
	}

	// Check AUR for foreign updates
	foreignPackages := libalpm.GetForeign()
	if len(foreignPackages) != 0 {
		err := aur.UpdateRemoteVersions(foreignPackages)
		if err != nil {
			fmt.Println("Error:", err)
		} else {
			var vercmp int
			for _, pkg := range foreignPackages {
				vercmp = alpm.VerCmp(pkg.LocalVersion, pkg.RemoteVersion)
				if pkg.RemoteVersion == "0" {
					fmt.Printf("Foreign package %s is not in AUR", pkg.Name)
					continue
				}
				if vercmp == -1 {
					fmt.Printf("[FOREIGN] %s %s -> %s\n",
						pkg.Name, pkg.LocalVersion, pkg.RemoteVersion)
				}
			}
		}
	}
}

func stringInList(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func pkgInList(haystack []*alpm.Pkg, needle *alpm.Pkg) bool {
	for _, v := range haystack {
		if v.Name == needle.Name {
			return true
		}
	}
	return false
}

func nameInPkgList(haystack []*alpm.Pkg, needle string) bool {
	for _, v := range haystack {
		if v.Name == needle {
			return true
		}
	}
	return false
}

func findPkgInList(haystack []*alpm.Pkg, needle *alpm.Pkg) (*alpm.Pkg, error) {
	for _, v := range haystack {
		if v.Name == needle.Name {
			return v, nil
		}
	}
	return nil, errors.New("Package not found in slice")
}

func systemArch() string {
	switch runtime.GOARCH {
	case "386":
		return "i686"
	case "amd64":
		return "x86_64"
	case "arm":
		return "arm"
	default:
		return "unknown"
	}
}
