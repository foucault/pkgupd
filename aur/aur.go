package aur

import "encoding/json"
import "net/http"
import "io/ioutil"
import "html"
import "errors"
import "strings"
import "pkgupd/alpm"

const RESP_TYPE_ERROR = "error"
const RESP_TYPE_MULTIINFO = "multiinfo"
const RESP_TYPE_MSEARCH = "msearch"
const RESP_TYPE_INFO = "info"
const RESP_TYPE_SEARCH = "search"
const QUERY_STRING = "https://aur.archlinux.org/rpc.php?type="

type aurResponse struct {
	Type    string          `json:"type"`
	Results json.RawMessage `json:"results"`
}

type AurPkg struct {
	Type           string `json:"URL"`
	Description    string `json:"Description"`
	Version        string `json:"Version"`
	Name           string `json:"Name"`
	FirstSubmitted int    `json:"FirstSubmitted"`
	License        string `json:"License"`
	ID             int    `json:"ID"`
	OutOFDate      int    `json:"OutOfDate"`
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

func InfoStr(pkg string) (*AurPkg, error) {
	queryString := QUERY_STRING + RESP_TYPE_INFO + `&`
	escPkg := `arg=` + html.EscapeString(pkg)
	finalUrl := queryString + escPkg
	response, err := getAurResponse(finalUrl)
	if err != nil {
		return nil, err
	}
	if response.Type == RESP_TYPE_ERROR {
		return nil, errors.New(string(response.Results))
	}
	if response.Type != RESP_TYPE_INFO {
		return nil, errors.New("Unexpected response type")
	}

	var aurPkg *AurPkg

	err = json.Unmarshal(response.Results, &aurPkg)
	if err != nil {
		return nil, err
	}

	return aurPkg, nil
}

func InfoPkg(pkg *alpm.Pkg) (*AurPkg, error) {
	return InfoStr(pkg.Name)
}

func UpdateRemoteVersions(fpkgs []*alpm.Pkg) error {
	// Morph packages into map for easy indexing
	pkgs := make(map[string]*alpm.Pkg)
	for _, p := range fpkgs {
		pkgs[p.Name] = p
	}
	if len(pkgs) == 0 {
		return errors.New("Package list is empty")
	}
	queryString := QUERY_STRING + RESP_TYPE_MULTIINFO + `&`
	var escPkgs []string
	for k, _ := range pkgs {
		escPkgs = append(escPkgs, "arg[]="+html.EscapeString(k))
	}
	finalUrl := queryString + strings.Join(escPkgs, "&")

	response, err := getAurResponse(finalUrl)
	if err != nil {
		return err
	}

	if response.Type == RESP_TYPE_ERROR {
		return errors.New(string(response.Results))
	}
	if response.Type != RESP_TYPE_MULTIINFO {
		return errors.New("Unexpected response type")
	}
	var aurPkgList []*AurPkg

	err = json.Unmarshal(response.Results, &aurPkgList)
	if err != nil {
		return err
	}

	var remVersion string
	for _, item := range aurPkgList {
		//fmt.Printf("%s: %s -> %s", item.Name)
		remVersion = item.Version
		pkgs[item.Name].RemoteVersion = remVersion
	}

	return nil
}
