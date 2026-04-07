package cli53

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestParseCharacterString(t *testing.T) {
	assert.Equal(t, "", parseCharacterString(""))
	assert.Equal(t, "abc", parseCharacterString("abc"))
	assert.Equal(t, "abc", parseCharacterString(`"abc"`))
	assert.Equal(t, "abc def", parseCharacterString(`"abc def"`))
	assert.Equal(t, `abc" def`, parseCharacterString(`"abc\" def"`))
	assert.Equal(t, `abc\ def`, parseCharacterString(`"abc\\ def"`))
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

func TestLoadAWSProfileSettings(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")
	content := `[default]
region = us-east-1
login_session = arn:aws:iam::111111111111:user/test

[profile demo]
region = us-west-2
role_arn = arn:aws:iam::222222222222:role/demo
source_profile = default
external_id = ext-123
role_session_name = demo-session
duration_seconds = 3600
`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))
	t.Setenv("AWS_CONFIG_FILE", configPath)

	source, found, err := loadAWSProfileSettings("default")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "us-east-1", source.Region)
	assert.Equal(t, "arn:aws:iam::111111111111:user/test", source.LoginSession)

	target, found, err := loadAWSProfileSettings("demo")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "us-west-2", target.Region)
	assert.Equal(t, "arn:aws:iam::222222222222:role/demo", target.RoleARN)
	assert.Equal(t, "default", target.SourceProfileName)
	assert.Equal(t, "ext-123", target.ExternalID)
	assert.Equal(t, "demo-session", target.RoleSessionName)
	assert.Equal(t, time.Hour, target.Duration)
}
