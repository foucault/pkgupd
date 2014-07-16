package alpm

import "testing"
import "strings"
import "fmt"
import "strconv"

const testcases = `
1.5.0 1.5.0  0
1.5.1 1.5.0  1
1.5.1 1.5    1
1.5.0-1 1.5.0-1  0
1.5.0-1 1.5.0-2 -1
1.5.0-1 1.5.1-1 -1
1.5.0-2 1.5.1-1 -1
1.5-1   1.5.1-1 -1
1.5-2   1.5.1-1 -1
1.5-2   1.5.1-2 -1
1.5   1.5-1 0
1.5-1 1.5   0
1.1-1 1.1   0
1.0-1 1.1  -1
1.1-1 1.0   1
1.5b-1  1.5-1  -1
1.5b    1.5    -1
1.5b-1  1.5    -1
1.5b    1.5.1  -1
1.0a     1.0alpha -1
1.0alpha 1.0b     -1
1.0b     1.0beta  -1
1.0beta  1.0rc    -1
1.0rc    1.0      -1
1.5.a    1.5     1
1.5.b    1.5.a   1
1.5.1    1.5.b   1
1.5.b-1  1.5.b   0
1.5-1    1.5.b  -1
2.0    2_0     0
2.0_a  2_0.a   0
2.0a   2.0.a  -1
2___a  2_a     1
0:1.0    0:1.0   0
0:1.0    0:1.1  -1
1:1.0    0:1.0   1
1:1.0    0:1.1   1
1:1.0    2:1.1  -1
1:1.0    0:1.0-1  1
1:1.0-1  0:1.1-1  1
0:1.0    1.0   0
0:1.0    1.1  -1
0:1.1    1.0   1
1:1.0    1.0   1
1:1.0    1.1   1
1:1.1    1.1   1
2.8.4-1  2.10.2-1 -1
5282-1   5275-1 1
5202-1   5275-1 -1
5300-1   5275-1 1
9.1-1   10.0-1 -1
9.1-1   1a.0-1 1
530-1   1247-1 -1`

func doCompare(t *testing.T, local string, remote string, expResult int64) int {

	wrn := "\033[31m\033[1m"
	wok := "\033[32m\033[1m"
	rst := "\033[0m"
	res := 0
	actResult := int64(VerCmp(local, remote))
	if actResult != expResult {
		fmt.Printf("%sFAIL   %s: '%s' vs '%s' should be '%d' but I got '%d'\n",
			wrn, rst, local, remote, expResult, actResult)
		t.Fail()
		res++
	} else {
		fmt.Printf("%sSUCCESS%s: '%s' vs '%s' is '%d'\n",
			wok, rst, local, remote, expResult)
	}
	fmt.Println("------")
	actResult = int64(VerCmp(remote, local))
	if actResult != -expResult {
		fmt.Printf("%sFAIL   %s: '%s' vs '%s' should be '%d' but I got '%d'\n",
			wrn, rst, remote, local, -expResult, actResult)
		t.Fail()
		res++
	} else {
		fmt.Printf("%sSUCCESS%s: '%s' vs '%s' is '%d'\n",
			wok, rst, remote, local, -expResult)
	}

	return res

}

func TestVerCmp(t *testing.T) {
	var local string
	var remote string
	var expResult int64
	//var actResult int64
	/*wrn := "\033[31m\033[1m"
	wok := "\033[32m\033[1m"
	rst := "\033[0m"*/
	lines := strings.Split(testcases, "\n")
	failed := 0
	total := 0
	fmt.Println("========")
	for _, line := range lines {
		line = strings.Trim(line, " \t\r\n")
		if len(line) == 0 {
			continue
		} else {
			parts := strings.Fields(line)
			local = parts[0]
			remote = parts[1]
			expResult, _ = strconv.ParseInt(parts[2], 10, 0)
			fmt.Printf("Comparing '%s' vs '%s'\n", local, remote)
			failed = failed + doCompare(t, local, remote, expResult)
			total = total + 2
		}
		fmt.Println("========")
	}
	if failed > 0 {
		fmt.Println("Failed", failed, "out of", total, "tests")
	} else {
		fmt.Println("All", total, "tests were successful")
	}
	fmt.Println()
}
