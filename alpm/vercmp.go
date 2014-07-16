package alpm

//import "fmt"
import "unicode"
import "strings"

func iMin(a int, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func iMax(a int, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

func isValidSeparator(x rune) bool {
	return unicode.IsPunct(x) || unicode.IsSpace(x)
}

func countClassRunes(s string, f func(rune) bool) int {
	count := 0
	for _, rune := range s {
		if f(rune) {
			count++
		}
	}
	return count
}

func parseEVR(evr string) (string, string, string) {
	var epoch string
	var version string
	var release string

	// Try to find epoch first
	eparts := strings.Split(evr, ":")
	var parts []string
	if len(eparts) == 1 {
		epoch = "0"
		parts = strings.Split(eparts[0], "-")
	} else {
		epoch = eparts[0]
		parts = strings.Split(eparts[1], "-")
	}

	version = parts[0]
	if len(parts) > 1 {
		release = parts[1]
	} else {
		release = "-1"
	}

	return epoch, version, release
}

func runesCmp(a rune, b rune) int {
	retval := 0
	cpa := int(a)
	cpb := int(b)
	//fmt.Printf("cpa: %d %q, cpb: %d %q\n", cpa, a, cpb, b)
	if unicode.IsLetter(a) || unicode.IsSpace(a) {
		//fmt.Println("a is letter/space")
		if unicode.IsLetter(b) || unicode.IsSpace(b) {
			//fmt.Println("b is letter/space")
			if cpa < cpb {
				retval = -1
			} else if cpa > cpb {
				retval = 1
			} else {
				retval = 0
			}
		} else if unicode.IsDigit(b) || b == '!' { // always ! > letter
			//fmt.Println("b is digit or !")
			retval = -1
		}
	} else if unicode.IsDigit(a) {
		//fmt.Println("a is digit")
		if unicode.IsLetter(b) || unicode.IsSpace(b) || b == '!' {
			//fmt.Println("b is letter/space")
			retval = 1
		} else if unicode.IsDigit(b) {
			//fmt.Println("b is digit")
			if cpa < cpb {
				retval = -1
			} else if cpa > cpb {
				retval = 1
			} else {
				retval = 0
			}
		} else if b == '!' {
			return 1
		}
	} else if a == '!' {
		if unicode.IsLetter(b) || unicode.IsSpace(b) {
			return 1
		} else if unicode.IsDigit(b) {
			return -1
		}
	}
	return retval
}

func bothLetters(a rune, b rune) bool {
	if unicode.IsLetter(a) && unicode.IsLetter(b) {
		return true
	}
	return false
}

func bothDigits(a rune, b rune) bool {
	if unicode.IsDigit(a) && unicode.IsDigit(b) {
		return true
	}
	return false
}

/*
 -1 if a < b
  0 if a = b
  1 if a > b
*/
func segmentCmp(a string, b string) int {
	//fmt.Printf("Comparing segments '%s' '%s'\n", a, b)
	lena := len(a)
	lenb := len(b)
	pointsToCheck := iMax(lena, lenb)
	retval := 0
	pointsChecked := 0

	var r1 rune
	var r2 rune
	canBreak := false
	for i := 0; i < pointsToCheck; i++ {
		pointsChecked++
		if retval != 0 && (bothLetters(r1, r2) || !bothDigits(r1, r2)) {
			// if (previous) runes are both letters or letter/digit break
			// no need to check further
			break
		}
		if lena < i+1 {
			// if previous rune is digit, pad with !
			if unicode.IsDigit(r1) {
				r1 = rune('!') // ! is a placeholder for missing digits
			} else { // else with spaces
				r1 = rune(' ')
			}
		} else {
			r1 = rune(a[i])
		}
		if lenb < i+1 { // same as above
			if unicode.IsDigit(r2) {
				r2 = rune('!')
			} else {
				r2 = rune(' ')
			}
		} else {
			r2 = rune(b[i])
		}
		runeRes := runesCmp(r1, r2)
		//fmt.Printf("%q %q -> %d\n", r1, r2, runeRes)
		if runeRes != 0 {
			retval = runeRes
			if (pointsChecked == pointsToCheck) || canBreak || lena == lenb {
				break
			}
		}
	}

	if retval == 0 && lena != lenb {
		if lena > lenb {
			retval = 1
		} else if lena < lenb {
			retval = -1
		}
	}

	return retval
}

/*
 -1 if a < b
  0 if a = b
  1 if a > b
*/
func rpmVerCmp(a string, b string) int {
	// Simplest case, strings are equal
	if a == b {
		return 0
	}
	// Split versions in segments
	segs1 := strings.FieldsFunc(a, isValidSeparator)
	segs2 := strings.FieldsFunc(b, isValidSeparator)
	//fmt.Println(segs1, segs2)
	cmp := 0

	fieldsToCheck := iMin(len(segs1), len(segs2))
	for i := 0; i < fieldsToCheck; i++ {
		scmp := segmentCmp(segs1[i], segs2[i])
		//fmt.Printf("Segments: %q %q -> %d\n", segs1[i], segs2[i], scmp)
		if scmp == -1 {
			cmp = -1
			break
		} else if scmp == 1 {
			cmp = 1
			break
		}
	}

	if cmp == 0 {
		if len(segs1) > len(segs2) {
			cmp = 1
		} else if len(segs1) < len(segs2) {
			cmp = -1
		}
		countSepsA := countClassRunes(a, isValidSeparator)
		countSepsB := countClassRunes(b, isValidSeparator)
		if countSepsA > countSepsB {
			cmp = 1
		} else if countSepsA < countSepsB {
			cmp = -1
		}
	}

	return cmp
}

/*
 -1 if a < b
  0 if a = b
  1 if a > b
*/
func VerCmp(a string, b string) int {
	if a == b {
		return 0
	}
	e1, v1, r1 := parseEVR(a)
	e2, v2, r2 := parseEVR(b)
	ret := rpmVerCmp(e1, e2)
	if ret == 0 {
		ret = rpmVerCmp(v1, v2)
		if ret == 0 && r1 != "-1" && r2 != "-1" {
			ret = rpmVerCmp(r1, r2)
		}
	}
	return ret
}

func PkgVerCmpLocal(a *Pkg, b *Pkg) int {
	return VerCmp(a.LocalVersion, b.LocalVersion)
}

func PkgVerCmpRemote(a *Pkg, b *Pkg) int {
	return VerCmp(a.RemoteVersion, b.RemoteVersion)
}

func IsUpdatablePkg(pkg *Pkg) bool {
	if pkg.RemoteVersion == "0" {
		// Not available, because no remote available
		return false
	}
	if VerCmp(pkg.LocalVersion, pkg.RemoteVersion) < 0 {
		return true
	}

	return false
}
