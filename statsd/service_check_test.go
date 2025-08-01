package statsd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func encodeSC(sc *ServiceCheck) (string, error) {
	err := sc.Check()
	if err != nil {
		return "", err
	}
	var buffer []byte
	buffer = appendServiceCheck(buffer, sc, nil, true)
	return string(buffer), nil
}

func TestServiceChecks(t *testing.T) {
	matrix := []struct {
		serviceCheck   *ServiceCheck
		expectedEncode string
	}{
		{
			NewServiceCheck("DataCatService", Ok),
			`_sc|DataCatService|0`,
		}, {
			NewServiceCheck("DataCatService", Warn),
			`_sc|DataCatService|1`,
		}, {
			NewServiceCheck("DataCatService", Critical),
			`_sc|DataCatService|2`,
		}, {
			NewServiceCheck("DataCatService", Unknown),
			`_sc|DataCatService|3`,
		}, {
			&ServiceCheck{Name: "DataCatService", Status: Ok, Hostname: "DataStation.Cat"},
			`_sc|DataCatService|0|h:DataStation.Cat`,
		}, {
			&ServiceCheck{Name: "DataCatService", Status: Ok, Hostname: "DataStation.Cat", Message: "Here goes valuable message"},
			`_sc|DataCatService|0|h:DataStation.Cat|m:Here goes valuable message`,
		}, {
			&ServiceCheck{Name: "DataCatService", Status: Ok, Hostname: "DataStation.Cat", Message: "Here are some cyrillic chars: к л м н о п р с т у ф х ц ч ш"},
			`_sc|DataCatService|0|h:DataStation.Cat|m:Here are some cyrillic chars: к л м н о п р с т у ф х ц ч ш`,
		}, {
			&ServiceCheck{Name: "DataCatService", Status: Ok, Hostname: "DataStation.Cat", Message: "Here goes valuable message", Tags: []string{"host:foo", "app:bar"}},
			`_sc|DataCatService|0|h:DataStation.Cat|#host:foo,app:bar|m:Here goes valuable message`,
		}, {
			&ServiceCheck{Name: "DataCatService", Status: Ok, Hostname: "DataStation.Cat", Message: "Here goes \n that should be escaped", Tags: []string{"host:foo", "app:b\nar"}},
			`_sc|DataCatService|0|h:DataStation.Cat|#host:foo,app:bar|m:Here goes \n that should be escaped`,
		}, {
			&ServiceCheck{Name: "DataCatService", Status: Ok, Hostname: "DataStation.Cat", Message: "Here goes m: that should be escaped", Tags: []string{"host:foo", "app:bar"}},
			`_sc|DataCatService|0|h:DataStation.Cat|#host:foo,app:bar|m:Here goes m\: that should be escaped`,
		},
	}

	for _, m := range matrix {
		scEncoded, err := encodeSC(m.serviceCheck)
		require.NoError(t, err)
		assert.Equal(t, m.expectedEncode, scEncoded)
	}

}

func TestNameMissing(t *testing.T) {
	sc := NewServiceCheck("", Ok)
	_, err := encodeSC(sc)
	require.Error(t, err)
	assert.Equal(t, "statsd.ServiceCheck name is required", err.Error())
}

func TestUnknownStatus(t *testing.T) {
	sc := NewServiceCheck("sc", ServiceCheckStatus(5))
	_, err := encodeSC(sc)
	require.Error(t, err)
	assert.Equal(t, "statsd.ServiceCheck status has invalid value", err.Error())
}

func TestNewServiceCheckWithTags(t *testing.T) {
	sc := NewServiceCheck("hello", Warn)
	sc.Tags = []string{"tag1", "tag2"}
	s, err := encodeSC(sc)
	require.NoError(t, err)
	assert.Equal(t, "_sc|hello|1|#tag1,tag2", s)
	assert.Len(t, sc.Tags, 2)
}

func TestNewServiceCheckWithTagsAppend(t *testing.T) {
	sc := NewServiceCheck("hello", Warn)
	sc.Tags = append(sc.Tags, "tag1", "tag2")
	s, err := encodeSC(sc)
	require.NoError(t, err)
	assert.Equal(t, "_sc|hello|1|#tag1,tag2", s)
	assert.Len(t, sc.Tags, 2)
}
