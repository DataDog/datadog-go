package statsd

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentTagOptOuts(t *testing.T) {
	for _, env := range []struct{ name, value string }{
		{"DD_ENTITY_ID", "agent-pod"},
		{"DD_ENV", "agent-env"},
		{"DD_SERVICE", "agent-service"},
		{"DD_VERSION", "agent-version"},
	} {
		old, exists := os.LookupEnv(env.name)
		defer func(name, old string, exists bool) {
			if exists {
				os.Setenv(name, old)
			} else {
				os.Unsetenv(name)
			}
		}(env.name, old, exists)
		require.NoError(t, os.Setenv(env.name, env.value))
	}
	withoutOriginGlobals(t)
	defer resetContainerID()

	allTags := []string{"dd.internal.entity_id:agent-pod", "env:agent-env", "service:agent-service", "version:agent-version"}
	targetTags := []string{"dd.internal.entity_id:target-pod", "env:target-env", "service:target-service", "version:target-version"}
	for _, tc := range []struct {
		name    string
		options []Option
		tags    []string
	}{
		{"default", nil, allTags},
		{"without_entity_id", []Option{WithoutEntityID()}, allTags[1:]},
		{"without_env_tags", []Option{WithoutEnvTags()}, allTags[:1]},
		{"without_both", []Option{WithoutEntityID(), WithoutEnvTags()}, nil},
		{"without_both_reversed", []Option{WithoutEnvTags(), WithoutEntityID()}, nil},
		{"explicit_global_tags", []Option{WithoutEntityID(), WithoutEnvTags(), WithTags(targetTags)}, targetTags},
		{"explicit_global_tags_first", []Option{WithTags(targetTags), WithoutEntityID(), WithoutEnvTags()}, targetTags},
		{"default_again", nil, allTags},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := append([]Option{WithoutTelemetry(), WithoutOriginDetection(), WithWorkersCount(1)}, tc.options...)
			var writer statsdWriterWrapper
			client, err := NewWithWriter(&writer, options...)
			require.NoError(t, err)
			defer client.Close()
			assert.ElementsMatch(t, tc.tags, client.clientEx.tags)

			ts := &testServer{tags: strings.Join(tc.tags, ",")}
			expected := ts.sendAllType(client)
			require.NoError(t, client.Close())
			ts.assertMetric(t, writer.data, expected)
		})
	}

	t.Run("explicit_metric_tags", func(t *testing.T) {
		var writer statsdWriterWrapper
		client, err := NewWithWriter(&writer, WithoutTelemetry(), WithoutOriginDetection(),
			WithoutEntityID(), WithoutEnvTags(), WithWorkersCount(1))
		require.NoError(t, err)
		defer client.Close()

		tags := []string{"dd.internal.entity_id:target-pod", "env:target-env", "service:target-service", "version:target-version"}
		require.NoError(t, client.Count("orders", 1, tags, 1))
		require.NoError(t, client.Close())
		assert.Equal(t, []string{"orders:1|c|#" + strings.Join(tags, ",")}, writer.data)
	})

	t.Run("clone", func(t *testing.T) {
		client, err := New("localhost:8125", WithoutTelemetry(), WithoutOriginDetection(),
			WithoutEntityID(), WithoutEnvTags())
		require.NoError(t, err)
		defer client.Close()

		cloned, err := CloneWithExtraOptions(client, WithTags([]string{"team:checkout"}))
		require.NoError(t, err)
		defer cloned.Close()
		assert.Equal(t, []string{"team:checkout"}, cloned.clientEx.tags)
	})
}
