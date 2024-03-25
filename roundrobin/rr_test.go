package roundrobin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/forward"
	"github.com/vulcand/oxy/v2/testutils"
	"github.com/vulcand/oxy/v2/utils"
)

func TestNoServers(t *testing.T) {
	fwd := forward.New(false)

	lb, err := New(fwd)
	require.NoError(t, err)

	proxy := httptest.NewServer(lb)
	t.Cleanup(proxy.Close)

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, re.StatusCode)
}

func TestRemoveBadServer(t *testing.T) {
	lb, err := New(nil)
	require.NoError(t, err)

	require.Error(t, lb.RemoveServer(testutils.ParseURI("http://google.com")))
}

func TestCustomErrHandler(t *testing.T) {
	errHandler := utils.ErrorHandlerFunc(func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(http.StatusText(http.StatusTeapot)))
	})

	fwd := forward.New(false)

	lb, err := New(fwd, ErrorHandler(errHandler))
	require.NoError(t, err)

	proxy := httptest.NewServer(lb)
	t.Cleanup(proxy.Close)

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusTeapot, re.StatusCode)
}

func TestOneServer(t *testing.T) {
	a := testutils.NewResponder("a")
	defer a.Close()

	fwd := forward.New(false)

	lb, err := New(fwd)
	require.NoError(t, err)

	require.NoError(t, lb.UpsertServer(testutils.ParseURI(a.URL)))

	proxy := httptest.NewServer(lb)
	t.Cleanup(proxy.Close)

	assert.Equal(t, []string{"a", "a", "a"}, seq(t, proxy.URL, 3))
}

func TestSimple(t *testing.T) {
	a := testutils.NewResponder("a")
	defer a.Close()

	b := testutils.NewResponder("b")
	defer b.Close()

	fwd := forward.New(false)

	lb, err := New(fwd)
	require.NoError(t, err)

	require.NoError(t, lb.UpsertServer(testutils.ParseURI(a.URL)))
	require.NoError(t, lb.UpsertServer(testutils.ParseURI(b.URL)))

	proxy := httptest.NewServer(lb)
	t.Cleanup(proxy.Close)

	assert.Equal(t, []string{"a", "b", "a"}, seq(t, proxy.URL, 3))
}

func TestRemoveServer(t *testing.T) {
	a := testutils.NewResponder("a")
	defer a.Close()

	b := testutils.NewResponder("b")
	defer b.Close()

	fwd := forward.New(false)

	lb, err := New(fwd)
	require.NoError(t, err)

	require.NoError(t, lb.UpsertServer(testutils.ParseURI(a.URL)))
	require.NoError(t, lb.UpsertServer(testutils.ParseURI(b.URL)))

	proxy := httptest.NewServer(lb)
	t.Cleanup(proxy.Close)

	assert.Equal(t, []string{"a", "b", "a"}, seq(t, proxy.URL, 3))

	err = lb.RemoveServer(testutils.ParseURI(a.URL))
	require.NoError(t, err)

	assert.Equal(t, []string{"b", "b", "b"}, seq(t, proxy.URL, 3))
}

func TestUpsertSame(t *testing.T) {
	a := testutils.NewResponder("a")
	defer a.Close()

	fwd := forward.New(false)

	lb, err := New(fwd)
	require.NoError(t, err)

	require.NoError(t, lb.UpsertServer(testutils.ParseURI(a.URL)))
	require.NoError(t, lb.UpsertServer(testutils.ParseURI(a.URL)))

	proxy := httptest.NewServer(lb)
	t.Cleanup(proxy.Close)

	assert.Equal(t, []string{"a", "a", "a"}, seq(t, proxy.URL, 3))
}

func TestUpsertWeight(t *testing.T) {
	a := testutils.NewResponder("a")
	defer a.Close()

	b := testutils.NewResponder("b")
	defer b.Close()

	fwd := forward.New(false)

	lb, err := New(fwd)
	require.NoError(t, err)

	require.NoError(t, lb.UpsertServer(testutils.ParseURI(a.URL)))
	require.NoError(t, lb.UpsertServer(testutils.ParseURI(b.URL)))

	proxy := httptest.NewServer(lb)
	t.Cleanup(proxy.Close)

	assert.Equal(t, []string{"a", "b", "a"}, seq(t, proxy.URL, 3))

	require.NoError(t, lb.UpsertServer(testutils.ParseURI(b.URL), Weight(3)))

	assert.Equal(t, []string{"b", "b", "a", "b"}, seq(t, proxy.URL, 4))
}

func TestWeighted(t *testing.T) {
	require.NoError(t, SetDefaultWeight(0))
	defer func() { _ = SetDefaultWeight(1) }()

	a := testutils.NewResponder("a")
	defer a.Close()

	b := testutils.NewResponder("b")
	defer b.Close()

	z := testutils.NewResponder("z")
	defer z.Close()

	fwd := forward.New(false)

	lb, err := New(fwd)
	require.NoError(t, err)

	require.NoError(t, lb.UpsertServer(testutils.ParseURI(a.URL), Weight(3)))
	require.NoError(t, lb.UpsertServer(testutils.ParseURI(b.URL), Weight(2)))
	require.NoError(t, lb.UpsertServer(testutils.ParseURI(z.URL), Weight(0)))

	proxy := httptest.NewServer(lb)
	t.Cleanup(proxy.Close)

	assert.Equal(t, []string{"a", "a", "b", "a", "b", "a"}, seq(t, proxy.URL, 6))

	w, ok := lb.ServerWeight(testutils.ParseURI(a.URL))
	assert.Equal(t, 3, w)
	assert.True(t, ok)

	w, ok = lb.ServerWeight(testutils.ParseURI(b.URL))
	assert.Equal(t, 2, w)
	assert.True(t, ok)

	w, ok = lb.ServerWeight(testutils.ParseURI(z.URL))
	assert.Equal(t, 0, w)
	assert.True(t, ok)

	w, ok = lb.ServerWeight(testutils.ParseURI("http://caramba:4000"))
	assert.Equal(t, -1, w)
	assert.False(t, ok)
}

func TestRequestRewriteListener(t *testing.T) {
	a := testutils.NewResponder("a")
	defer a.Close()

	b := testutils.NewResponder("b")
	defer b.Close()

	fwd := forward.New(false)

	lb, err := New(fwd,
		RoundRobinRequestRewriteListener(func(_ *http.Request, _ *http.Request) {}))
	require.NoError(t, err)

	assert.NotNil(t, lb.requestRewriteListener)
}

func seq(t *testing.T, url string, repeat int) []string {
	t.Helper()

	var out []string
	for i := 0; i < repeat; i++ {
		_, body, err := testutils.Get(url)
		require.NoError(t, err)
		out = append(out, string(body))
	}
	return out
}
