package alpm

import "unicode"
import "strings"

// Returns the minimum integer
func iMin(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

// Returns the maximum integer
func iMax(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

// Checks if the rune x is a valid version separator,
// basically if it is a punctuation symbol or a space
func isValidSeparator(x rune) bool {
	return unicode.IsPunct(x) || unicode.IsSpace(x)
}

// Counts the runes of string s that fulfill the the f()
// function
func countClassRunes(s string, f func(rune) bool) int {
	count := 0
	for _, rune := range s {
		if f(rune) {
			count++
		}
	}
	return count
}

// Splits a version string in epoch, version and release
// For example 1:5.12-3 returns (1, 5.12, 3). If no
// epoch is available it defaults to 0.
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

// Find which rune is "greater" in a semantic
// versioning sense.
func runesCmp(a rune, b rune) int {
	retval := 0
	// Make rune into codepoints
	cpa := int(a)
	cpb := int(b)
	//fmt.Printf("cpa: %d %q, cpb: %d %q\n", cpa, a, cpb, b)
	// First check if a is a letter/space, or digit/!
	if unicode.IsLetter(a) || unicode.IsSpace(a) {
		//fmt.Println("a is letter/space")
		// Then if b is a letter or space
		if unicode.IsLetter(b) || unicode.IsSpace(b) {
			//fmt.Println("b is letter/space")
			// Directly compare their codepoints
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
			// Digits are always greater than letters/spaces/!
			retval = 1
		} else if unicode.IsDigit(b) {
			//fmt.Println("b is digit")
			// Directly compare their codepoints
			if cpa < cpb {
				retval = -1
			} else if cpa > cpb {
				retval = 1
			} else {
				retval = 0
			}
		} else if b == '!' {
			// digits are always greater than !
			return 1
		}
	} else if a == '!' { // Check if a is a placeholder
		if unicode.IsLetter(b) || unicode.IsSpace(b) {
			// ! is always greater than letters
			return 1
		} else if unicode.IsDigit(b) {
			// but always less than digits
			return -1
		}
	}
	return retval
}

// Returns true if both runs are letters, false otherwise
func bothLetters(a rune, b rune) bool {
	if unicode.IsLetter(a) && unicode.IsLetter(b) {
		return true
	}
	return false
}

// Returns true if both runes are digits, false otherwise
func bothDigits(a rune, b rune) bool {
	if unicode.IsDigit(a) && unicode.IsDigit(b) {
		return true
	}
	return false
}

// Compare two segment strings. Segment strings are the
// numbers or letters that form the version. For example
// 4.12.53.2 has four segments: 4, 12, 53, 2. Instead of
// a period any separator is compatible. Each rune of the
// segment is checked until a non zero comparison value is
// encountered. The final segments are always the same length
// and the missing digits/letters are filled with ! or ' '
// based on the following rule
// 1. If previous rune is digit, fill with !
// 2. If previous rune is letter, fill with ' '
// 51 and 5 -> "51" and "5!"
// 1a and 1 -> "1a" and "10" (vercmp 1a 1 is -1)
// 1ab and 1a -> "1ab" and "1a " (vercmp 1ab 1a is 1)
// 1alpha and 1a -> "1alpha" and "1a    "
func segmentCmp(a string, b string) int {
	//fmt.Printf("Comparing segments '%s' '%s'\n", a, b)
	lena := len(a)
	lenb := len(b)
	// The runes are always the maximum of the length
	// of the two strings
	pointsToCheck := iMax(lena, lenb)
	retval := 0
	pointsChecked := 0

	var r1 rune
	var r2 rune
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
			} else { // else with spaces which is a placeholder for missing letters
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
		if runeRes != 0 { // If runes are not equal
			retval = runeRes
			// Stop looping through the loop only if
			// 1. Reached the maximum number of code points to check
			// OR
			// 2. segments have the same length
			if (pointsChecked == pointsToCheck) || lena == lenb {
				break
			}
		}
	}

	// If the return value after the loop is still 0
	// the longer string is alway greater
	if retval == 0 && lena != lenb {
		if lena > lenb {
			retval = 1
		} else if lena < lenb {
			retval = -1
		}
	}

	return retval
}

// Compare two version parts. The parts are split into
// segments and each segment is checked until a non zero
// value is encountered
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

// VerCmp compares two version strings. The functionality should
// be identically to the vercmp function of libalpm. The returned
// values are:
// -1 if a < b
//  0 if a = b
//  1 if a > b
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

// PkgVerCmpLocal compares the local versions of two packages
func PkgVerCmpLocal(a *Pkg, b *Pkg) int {
	return VerCmp(a.LocalVersion, b.LocalVersion)
}

// PkgVerCmpRemote compares the remote versions of two packages
func PkgVerCmpRemote(a *Pkg, b *Pkg) int {
	return VerCmp(a.RemoteVersion, b.RemoteVersion)
}

// IsUpdatablePkg checks if a package is updatable, which means
// if VerCmp(pkg.LocalVersion, pkg.LocalVersion) == -1. If the
// remote version of the package is 0 means that no remote package
// is available and therefore the package is not updatable.
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
