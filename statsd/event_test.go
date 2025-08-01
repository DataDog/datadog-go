package statsd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func encodeEvent(e *Event) (string, error) {
	err := e.Check()
	if err != nil {
		return "", err
	}
	var buffer []byte
	buffer = appendEvent(buffer, e, nil, true)
	return string(buffer), nil
}

func TestEventEncode(t *testing.T) {
	matrix := []struct {
		event   *Event
		encoded string
	}{
		{
			NewEvent("Hello", "Something happened to my event"),
			`_e{5,30}:Hello|Something happened to my event`,
		}, {
			&Event{Title: "hi", Text: "okay", AggregationKey: "foo"},
			`_e{2,4}:hi|okay|k:foo`,
		}, {
			&Event{Title: "hi", Text: "okay", AggregationKey: "foo", AlertType: Info},
			`_e{2,4}:hi|okay|k:foo|t:info`,
		}, {
			&Event{Title: "hi", Text: "w/e", AlertType: Error, Priority: Normal},
			`_e{2,3}:hi|w/e|p:normal|t:error`,
		}, {
			&Event{Title: "hi", Text: "uh", Tags: []string{"host:foo", "app:bar"}},
			`_e{2,2}:hi|uh|#host:foo,app:bar`,
		}, {
			&Event{Title: "hi", Text: "line1\nline2", Tags: []string{"hello\nworld"}},
			`_e{2,12}:hi|line1\nline2|#helloworld`,
		},
	}

	for _, m := range matrix {
		r, err := encodeEvent(m.event)
		require.NoError(t, err)
		assert.Equal(t, r, m.encoded)
	}
}

func TestNewEventTitleMissing(t *testing.T) {
	e := NewEvent("", "hi")
	_, err := encodeEvent(e)
	require.Error(t, err)
	assert.Equal(t, "statsd.Event title is required", err.Error())
}

func TestNewEvent(t *testing.T) {
	e := NewEvent("hello", "world")
	e.Tags = []string{"tag1", "tag2"}
	eventEncoded, err := encodeEvent(e)
	require.NoError(t, err)
	assert.Equal(t, "_e{5,5}:hello|world|#tag1,tag2", eventEncoded)
	assert.Len(t, e.Tags, 2)
}

func TestNewEventTagsAppend(t *testing.T) {
	e := NewEvent("hello", "world")
	e.Tags = append(e.Tags, "tag1", "tag2")
	eventEncoded, err := encodeEvent(e)
	require.NoError(t, err)
	assert.Equal(t, "_e{5,5}:hello|world|#tag1,tag2", eventEncoded)
	assert.Len(t, e.Tags, 2)
}

func TestNewEventEmptyText(t *testing.T) {
	e := NewEvent("hello", "")
	e.Tags = append(e.Tags, "tag1", "tag2")
	eventEncoded, err := encodeEvent(e)
	require.NoError(t, err)
	assert.Equal(t, "_e{5,0}:hello||#tag1,tag2", eventEncoded)
	assert.Len(t, e.Tags, 2)
}
