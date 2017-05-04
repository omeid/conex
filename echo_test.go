package conex_test

import (
	"testing"

	"github.com/omeid/conex/echo"
	echolib "github.com/omeid/echo"
)

func TestEcho1(t *testing.T) { t.Parallel(); echoTest(t, false) }
func TestEcho2(t *testing.T) { t.Parallel(); echoTest(t, false) }
func TestEcho3(t *testing.T) { t.Parallel(); echoTest(t, true) }
func TestEcho4(t *testing.T) { t.Parallel(); echoTest(t, true) }

func echoTest(t *testing.T, reverse bool) {

	e, c := echo.Box(t, reverse)
	defer c.Drop()

	cases := []string{
		"hello",
		"hi",
	}

	for i, say := range cases {
		expect := say
		if reverse {
			expect = echolib.Reverse(say)
		}

		reply, err := e.Say(say)

		if err != nil {
			t.Fatal(err)
		}

		if reply != expect {
			t.Fatalf("\nSaid: %s\nExpected: %s\nGot:      %s\n", say, expect, reply)
		}

		count, err := e.Count()
		if err != nil {
			t.Fatal(err)
		}

		c := int(count)
		if c != i+1 {
			t.Fatalf("\nCount:\nExpected: %v\nGot:      %v\n", i, c)
		}

	}

}

func BenchmarkEcho(b *testing.B) {

	reverse := false
	say := "hello"
	expect := say

	e, c := echo.Box(b, reverse)
	defer c.Drop()

	for n := 0; n < b.N; n++ {

		reply, err := e.Say(say)

		if err != nil {
			b.Fatal(err)
		}

		if reply != expect {
			b.Fatalf("\nSaid: %s\nExpected: %s\nGot:      %s\n", say, expect, reply)
		}
	}
}
