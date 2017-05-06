package cli53

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyValuesString(t *testing.T) {
	assert.Equal(t, "a=1", KeyValues{"a", 1}.String())
	assert.Equal(t, `a=""`, KeyValues{"a", ""}.String())
	assert.Equal(t, `a="1"`, KeyValues{"a", "1"}.String())
	assert.Equal(t, `a="\""`, KeyValues{"a", `"`}.String())
	assert.Equal(t, `a="\\"`, KeyValues{"a", `\`}.String())
}

func mustParse(input string) KeyValues {
	result, err := ParseKeyValues(input)
	if err != nil {
		panic(err)
	}
	return result
}

func TestParseKeyValuesValid(t *testing.T) {
	assert.Equal(t, KeyValues{"a", 1}, mustParse("a=1"))
	assert.Equal(t, KeyValues{"a", ""}, mustParse(`a=""`))
	assert.Equal(t, KeyValues{"identifier", 1}, mustParse("identifier=1"))
	assert.Equal(t, KeyValues{"mixedCase", 1}, mustParse("mixedCase=1"))
	assert.Equal(t, KeyValues{"a", "b"}, mustParse(`a="b"`))
	assert.Equal(t, KeyValues{"a", `b"c`}, mustParse(`a="b\"c"`))
	assert.Equal(t, KeyValues{"a", 1, "b", 2}, mustParse("a=1 b=2"))
	assert.Equal(t, KeyValues{"a", 1, "b", 2}, mustParse("a=1        b=2"))
	assert.Equal(t, KeyValues{"a", 1, "b", "c"}, mustParse(`a=1 b="c"`))
	assert.Equal(t, KeyValues{"a", 1, "b", "c d"}, mustParse(`a=1 b="c d"`))
	assert.Equal(t, KeyValues{"a", 1, "b", `c"\d`}, mustParse(`a=1 b="c\"\\d"`))
}

func parsingError(input string) error {
	_, err := ParseKeyValues(input)
	return err
}

func TestParseKeyValuesErrors(t *testing.T) {
	assert.Error(t, parsingError(""))
	assert.Error(t, parsingError("a"))
	assert.Error(t, parsingError("1"))
	assert.Error(t, parsingError("a="))
	assert.Error(t, parsingError(`a="`))
	assert.Error(t, parsingError(`a="\"`))
	assert.Error(t, parsingError(`a=1b=2`))
	assert.Error(t, parsingError(`a=x`))
}

func TestQuote(t *testing.T) {
	assert.Equal(t, `""`, quote(""))
	assert.Equal(t, `"a"`, quote("a"))
	assert.Equal(t, `"\"quo\\ted\""`, quote(`"quo\ted"`))
}

func TestSplitValues(t *testing.T) {
	assert.Equal(t, []string{}, splitValues(""))
	assert.Equal(t, []string{""}, splitValues(`""`))
	assert.Equal(t, []string{"abc"}, splitValues(`"abc"`))
	assert.Equal(t, []string{"abc", "def"}, splitValues(`"abc" "def"`))
	assert.Equal(t, []string{`a "quote" b`}, splitValues(`"a \"quote\" b"`))
}

func TestIsZoneId(t *testing.T) {
	assert.True(t, isZoneId("Z1DXU7RZRUQ"))
	assert.True(t, isZoneId("Z1DXU7RZRUQP"))
	assert.True(t, isZoneId("Z1DXU7RZRUQPI"))
	assert.True(t, isZoneId("Z1DXU7RZRUQPIP"))
	assert.True(t, isZoneId("/hostedzone/Z1DXU7RZRUQPIP"))
	assert.False(t, isZoneId("example.com"))
	assert.False(t, isZoneId("example.com."))
	assert.False(t, isZoneId("0.1.10.in-addr.arpa."))
}

func TestQualifyName(t *testing.T) {
	assert.Equal(t, "example.com.", qualifyName("", "example.com."))
	assert.Equal(t, "example.com.", qualifyName("@", "example.com."))
	assert.Equal(t, "a.example.com.", qualifyName("a", "example.com."))
	assert.Equal(t, "a.", qualifyName("a.", "example.com."))
	assert.Equal(t, "a.b.example.com.", qualifyName("a.b", "example.com."))
}

func TestShortenName(t *testing.T) {
	assert.Equal(t, "@", shortenName("example.com.", "example.com."))
	assert.Equal(t, "a", shortenName("a.example.com.", "example.com."))
	assert.Equal(t, "a.", shortenName("a.", "example.com."))
	assert.Equal(t, "a.b", shortenName("a.b.example.com.", "example.com."))
	assert.Equal(t, "fineexample.com.", shortenName("fineexample.com.", "example.com."))
}
