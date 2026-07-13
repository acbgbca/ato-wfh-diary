//go:build e2e || e2e_docker

package e2e_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-rod/rod"
)

// swCall is one side effect the service worker performed while handling a
// pushsubscriptionchange event: either a network request ("fetch") or a call to
// pushManager.subscribe ("subscribe").
type swCall struct {
	Kind            string         `json:"kind"`
	Method          string         `json:"method"`
	URL             string         `json:"url"`
	Body            map[string]any `json:"body"`
	UserVisibleOnly bool           `json:"user_visible_only"`
	Key             []int          `json:"key"`
}

// swResult is what the sandbox harness reports back after dispatching the event.
type swResult struct {
	Registered    bool     `json:"registered"`
	WaitUntilArgs int      `json:"wait_until_args"`
	Calls         []swCall `json:"calls"`
}

// The VAPID key the stubbed /api/notifications/vapid-key response hands back,
// and the bytes urlBase64ToUint8Array must decode it to. "BQID" is URL-safe
// base64 for the three bytes 0x05 0x02 0x03.
const (
	swVapidKey     = "BQID"
	swNewEndpoint  = "https://push.example/new"
	swOldEndpoint  = "https://push.example/old"
	swNewP256dhKey = "new-p256dh"
	swNewAuthKey   = "new-auth"
)

var swVapidKeyBytes = []int{5, 2, 3}

// dispatchPushSubscriptionChange loads the deployed /sw.js, runs it in a sandbox
// with stubbed service-worker globals, dispatches a pushsubscriptionchange event
// at it, and reports everything the worker did in response.
//
// The real service worker cannot be driven from a test: Chrome will not create a
// genuine push subscription without a push service, and there is no way to make
// the browser fire pushsubscriptionchange on demand. Evaluating the shipped
// source against fake globals exercises the worker's actual handler code, which
// is the part this test is about.
//
// oldEndpoint is the endpoint on event.oldSubscription; pass "" to simulate a
// browser that does not provide it.
func dispatchPushSubscriptionChange(t *testing.T, page *rod.Page, oldEndpoint string) swResult {
	t.Helper()

	raw := page.MustEval(`async (oldEndpoint, vapidKey, newEndpoint, p256dh, auth) => {
		const src = await (await fetch('/sw.js')).text();
		const calls = [];

		const fakeSub = {
			endpoint: newEndpoint,
			toJSON: () => ({ endpoint: newEndpoint, keys: { p256dh, auth } }),
		};

		const fakeSelf = {
			listeners: {},
			addEventListener(type, fn) {
				(this.listeners[type] = this.listeners[type] || []).push(fn);
			},
			skipWaiting() {},
			clients: { claim() {} },
			registration: {
				showNotification() {},
				pushManager: {
					subscribe(opts) {
						calls.push({
							kind: 'subscribe',
							user_visible_only: !!opts.userVisibleOnly,
							key: Array.from(opts.applicationServerKey || []),
						});
						return Promise.resolve(fakeSub);
					},
				},
			},
		};

		const fakeCaches = {
			open: () => Promise.resolve({ addAll: () => Promise.resolve() }),
			keys: () => Promise.resolve([]),
			match: () => Promise.resolve(undefined),
			delete: () => Promise.resolve(true),
		};

		const fakeFetch = (url, opts) => {
			opts = opts || {};
			calls.push({
				kind: 'fetch',
				method: opts.method || 'GET',
				url: String(url),
				body: opts.body ? JSON.parse(opts.body) : null,
			});
			if (String(url).includes('vapid-key')) {
				return Promise.resolve({
					ok: true, status: 200,
					json: () => Promise.resolve({ vapid_public_key: vapidKey }),
				});
			}
			return Promise.resolve({
				ok: true, status: 200,
				json: () => Promise.resolve({ status: 'ok' }),
			});
		};

		// The worker only calls self.addEventListener at the top level, so running
		// it against fake globals is enough to collect its handlers.
		new Function('self', 'caches', 'clients', 'fetch', src)(
			fakeSelf, fakeCaches, fakeSelf.clients, fakeFetch);

		const handlers = fakeSelf.listeners['pushsubscriptionchange'] || [];
		if (handlers.length === 0) return { registered: false, wait_until_args: 0, calls };

		const waited = [];
		const event = {
			waitUntil: p => waited.push(p),
			oldSubscription: oldEndpoint ? { endpoint: oldEndpoint } : null,
		};
		handlers[0](event);
		await Promise.all(waited);

		return { registered: true, wait_until_args: waited.length, calls };
	}`, oldEndpoint, swVapidKey, swNewEndpoint, swNewP256dhKey, swNewAuthKey)

	encoded, err := json.Marshal(raw.Val())
	if err != nil {
		t.Fatalf("marshal harness result: %v", err)
	}
	var result swResult
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatalf("decode harness result: %v", err)
	}
	return result
}

// findCall returns the first recorded call matching kind/method/url-substring.
func findCall(calls []swCall, kind, method, urlPart string) *swCall {
	for i := range calls {
		c := calls[i]
		if c.Kind != kind {
			continue
		}
		if kind == "subscribe" {
			return &calls[i]
		}
		if c.Method == method && strings.Contains(c.URL, urlPart) {
			return &calls[i]
		}
	}
	return nil
}

// TestE2E_SW_PushSubscriptionChange_ReSubscribes verifies that when the browser
// invalidates a push subscription, the service worker fetches the VAPID key,
// re-subscribes, registers the new subscription with the server, and deletes the
// old endpoint.
func TestE2E_SW_PushSubscriptionChange_ReSubscribes(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL)

	result := dispatchPushSubscriptionChange(t, page, swOldEndpoint)

	if !result.Registered {
		t.Fatal("sw.js does not register a pushsubscriptionchange listener")
	}
	if result.WaitUntilArgs != 1 {
		t.Errorf("event.waitUntil calls: got %d, want 1 (the worker may be killed before it finishes)", result.WaitUntilArgs)
	}

	if c := findCall(result.Calls, "fetch", "GET", "/api/notifications/vapid-key"); c == nil {
		t.Errorf("expected a GET of the VAPID key, got calls: %+v", result.Calls)
	}

	sub := findCall(result.Calls, "subscribe", "", "")
	if sub == nil {
		t.Fatalf("expected pushManager.subscribe to be called, got calls: %+v", result.Calls)
	}
	if !sub.UserVisibleOnly {
		t.Error("pushManager.subscribe must be called with userVisibleOnly: true")
	}
	if len(sub.Key) != len(swVapidKeyBytes) {
		t.Errorf("applicationServerKey: got %v, want the decoded VAPID key %v", sub.Key, swVapidKeyBytes)
	} else {
		for i, b := range swVapidKeyBytes {
			if sub.Key[i] != b {
				t.Errorf("applicationServerKey: got %v, want the decoded VAPID key %v", sub.Key, swVapidKeyBytes)
				break
			}
		}
	}

	post := findCall(result.Calls, "fetch", "POST", "/api/notifications/subscribe")
	if post == nil {
		t.Fatalf("expected the new subscription to be POSTed, got calls: %+v", result.Calls)
	}
	if post.Body["endpoint"] != swNewEndpoint {
		t.Errorf("POST endpoint: got %v, want %q", post.Body["endpoint"], swNewEndpoint)
	}
	if post.Body["p256dh_key"] != swNewP256dhKey {
		t.Errorf("POST p256dh_key: got %v, want %q", post.Body["p256dh_key"], swNewP256dhKey)
	}
	if post.Body["auth_key"] != swNewAuthKey {
		t.Errorf("POST auth_key: got %v, want %q", post.Body["auth_key"], swNewAuthKey)
	}

	del := findCall(result.Calls, "fetch", "DELETE", "/api/notifications/subscribe")
	if del == nil {
		t.Fatalf("expected the old endpoint to be DELETEd, got calls: %+v", result.Calls)
	}
	if del.Body["endpoint"] != swOldEndpoint {
		t.Errorf("DELETE endpoint: got %v, want the old endpoint %q", del.Body["endpoint"], swOldEndpoint)
	}
}

// TestE2E_SW_PushSubscriptionChange_NoOldSubscription verifies the worker still
// re-subscribes when the browser gives no oldSubscription, and does not send a
// DELETE with an empty endpoint (the server rejects that with 400).
func TestE2E_SW_PushSubscriptionChange_NoOldSubscription(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL)

	result := dispatchPushSubscriptionChange(t, page, "")

	if !result.Registered {
		t.Fatal("sw.js does not register a pushsubscriptionchange listener")
	}
	if post := findCall(result.Calls, "fetch", "POST", "/api/notifications/subscribe"); post == nil {
		t.Fatalf("expected the new subscription to be POSTed, got calls: %+v", result.Calls)
	}
	if del := findCall(result.Calls, "fetch", "DELETE", "/api/notifications/subscribe"); del != nil {
		t.Errorf("no oldSubscription, so no DELETE should be sent; got %+v", *del)
	}
}
