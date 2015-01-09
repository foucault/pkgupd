// Package aur provides functions and tools to interact with the
// ArchLinux User Repository
package aur

import "encoding/json"
import "net/http"
import "io/ioutil"
import "html"
import "errors"
import "strings"
import "pkgupd/alpm"

// AUR Response type error
const RespTypeError = "error"

// AUR Response type multiinfo
const RespTypeMultiinfo = "multiinfo"

// AUR Response type msearch
const RespTypeMsearch = "msearch"

// AUR Response type info
const RespTypeInfo = "info"

// AUR Response type search
const RespTypeSearch = "search"

// The base query url
const QueryString = "https://aur.archlinux.org/rpc.php?type="

type aurResponse struct {
	Type    string          `json:"type"`
	Results json.RawMessage `json:"results"`
}

// AurPkg is the type used to represent an AUR package and it
// is also used to unmarshal AUR responses
type Pkg struct {
	Type           string `json:"URL"`
	Description    string `json:"Description"`
	Version        string `json:"Version"`
	Name           string `json:"Name"`
	FirstSubmitted int    `json:"FirstSubmitted"`
	License        string `json:"License"`
	ID             int    `json:"ID"`
	OutOfDate      int    `json:"OutOfDate"`
	LastModified   int    `json:"LastModified"`
	Maintainer     string `json:"Maintainer"`
	CategoryID     int    `json:"CategoryID"`
	URLPath        string `json:"URLPath"`
	NumVotes       int    `json:"NumVotes"`
}

func getAurResponse(query string) (*aurResponse, error) {
	res, err := http.Get(query)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	res.Body.Close()
	var response aurResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// InfoStr searches the AUR for a package named pkg and returns
// its information as an *AurPkg struct. If an error is encountered
// the result will be nil and the error will be populated with the
// server's response
func InfoStr(pkg string) (*Pkg, error) {
	queryString := QueryString + RespTypeInfo + `&`
	escPkg := `arg=` + html.EscapeString(pkg)
	finalURL := queryString + escPkg
	response, err := getAurResponse(finalURL)
	if err != nil {
		return nil, err
	}
	if response.Type == RespTypeError {
		return nil, errors.New(string(response.Results))
	}
	if response.Type != RespTypeInfo {
		return nil, errors.New("Unexpected response type")
	}

	var aurPkg *Pkg

	err = json.Unmarshal(response.Results, &aurPkg)
	if err != nil {
		return nil, err
	}

	return aurPkg, nil
}

// InfoPkg is the same as InfoStr but uses an *alpm.Pkg as argument
func InfoPkg(pkg *alpm.Pkg) (*Pkg, error) {
	return InfoStr(pkg.Name)
}

// UpdateRemoteVersions will populate the RemoteVersion field of
// all the provided alpm.Pkg structs with their AUR version if
// available. If a server error is encountered the returned
// error contains the server's respose
func UpdateRemoteVersions(fpkgs []*alpm.Pkg) error {
	// Morph packages into map for easy indexing
	pkgs := make(map[string]*alpm.Pkg)
	for _, p := range fpkgs {
		pkgs[p.Name] = p
	}
	if len(pkgs) == 0 {
		return errors.New("Package list is empty")
	}
	queryString := QueryString + RespTypeMultiinfo + `&`
	var escPkgs []string
	for k := range pkgs {
		escPkgs = append(escPkgs, "arg[]="+html.EscapeString(k))
	}
	finalURL := queryString + strings.Join(escPkgs, "&")

	response, err := getAurResponse(finalURL)
	if err != nil {
		return err
	}

	if response.Type == RespTypeError {
		return errors.New(string(response.Results))
	}
	if response.Type != RespTypeMultiinfo {
		return errors.New("Unexpected response type")
	}
	var aurPkgList []*Pkg

	err = json.Unmarshal(response.Results, &aurPkgList)
	if err != nil {
		return err
	}

	var remVersion string
	for _, item := range aurPkgList {
		remVersion = item.Version
		pkgs[item.Name].RemoteVersion = remVersion
	}

	return nil
}
