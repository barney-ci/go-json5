package json5

import (
	"encoding/json"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestReaderValid(t *testing.T) {

	tcases := []struct{
		In, Out string
	}{
		{
			In: `
			{
				// Some comment
				// and another one
				"hello": "world"
			}
			`,
			Out: `
			{
				"hello": "world"
			}
			`,
		},
		{
			In: `
			// a comment
			{
				// another comment
				"hello": "world" // yet another one
				// one more
			}
			// a final one
			`,
			Out: `
			{
				"hello": "world"
			}
			`,
		},
		{
			In: `
			{
				hello: "world"
			}
			`,
			Out: `
			{
				"hello": "world"
			}
			`,
		},
		{
			In: `
			{
				num: 1,
				hex: 0xff,
				leading: .1234,
				trailing: 1234.,
				trailingExp: 1234.e-16,
				plus: +1,
				plusExp: 1e+1
			}
			`,
			Out: `
			{
				"num": 1,
				"hex": 255,
				"leading": 0.1234,
				"trailing": 1234.0,
				"trailingExp": 1234.0e-16,
				"plus": 1,
				"plusExp": 1e1
			}
			`,
		},
		{
			In: `
			{
				"hash": { "foo": "bar", },
				"array": [ "foo", ],
			}
			`,
			Out: `
			{
				"hash": { "foo": "bar" },
				"array": [ "foo" ]
			}
			`,
		},
		{
			In: `
			{
				"single": 'hello, world',
				"multiline": "\
hello, \
world",
			}
			`,
			Out: `
			{
				"single": "hello, world",
				"multiline": "\nhello, \nworld"
			}
			`,
		},
		{
			// This is the example on the official json5 page.
			In: `{
  // comments
  unquoted: 'and you can quote me on that',
  singleQuotes: 'I can use "double quotes" here',
  lineBreaks: "Look, Mom! \
No \\n's!",
  hexadecimal: 0xdecaf,
  leadingDecimalPoint: .8675309, andTrailing: 8675309.,
  positiveSign: +1,
  trailingComma: 'in objects', andIn: ['arrays',],
  "backwardsCompatible": "with JSON",
}`,
			Out: `
			{
				"unquoted": "and you can quote me on that",
				"singleQuotes": "I can use \"double quotes\" here",
				"lineBreaks": "Look, Mom! \nNo \\n's!",
				"hexadecimal": 912559,
				"leadingDecimalPoint": 0.8675309,
				"andTrailing": 8675309.0,
				"positiveSign": 1,
				"trailingComma": "in objects",
				"andIn": ["arrays"],
				"backwardsCompatible": "with JSON"
			}
			`,
		},
		{
			In: `
			{
				"foo": {
					"bar": "baz",
					"quxx": true,
				},
				"bar": {},
			}
			`,
			Out: `
			{
				"foo": {
					"bar": "baz",
					"quxx": true
				},
				"bar": {}
			}
			`,
		},
		{
			In: `
			[
				{
					"foo": "bar",
				},
				{
					"foo": "bar",
				},
			]
			`,
			Out: `
			[
				{
					"foo": "bar"
				},
				{
					"foo": "bar"
				}
			]
			`,
		},
		{
			In: `
			{
				"test": 0,
			}
			`,
			Out: `
			{
				"test": 0
			}
			`,
		},
	}

	for i, tc := range tcases {
		t.Run(strconv.Itoa(i), func (t *testing.T) {
			var expected, actual interface{}
			json.Unmarshal([]byte(tc.Out), &expected)
			err := Unmarshal([]byte(tc.In), &actual)
			if err != nil {
				txt, _ := io.ReadAll(NewReader(strings.NewReader(tc.In)))
				t.Fatalf("error %v (translated json: %v)", err, string(txt))
			}

			if !reflect.DeepEqual(expected, actual) {
				txt, _ := io.ReadAll(NewReader(strings.NewReader(tc.In)))
				t.Fatalf("expected %v, got %v (translated json: %v)", expected, actual, string(txt))
			}
		})
	}
}
