package qq

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenhg5/cc-connect/core"

	"github.com/gorilla/websocket"
)

func TestPlatform_Name(t *testing.T) {
	p := &Platform{}
	if got := p.Name(); got != "qq" {
		t.Errorf("Name() = %q, want %q", got, "qq")
	}
}

func TestNew_DefaultWSURL(t *testing.T) {
	p, err := New(map[string]any{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	platform := p.(*Platform)
	if platform.wsURL != "ws://127.0.0.1:3001" {
		t.Errorf("wsURL = %q, want %q", platform.wsURL, "ws://127.0.0.1:3001")
	}
}

func TestNew_CustomWSURL(t *testing.T) {
	p, err := New(map[string]any{
		"ws_url": "ws://example.com:8080",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	platform := p.(*Platform)
	if platform.wsURL != "ws://example.com:8080" {
		t.Errorf("wsURL = %q, want %q", platform.wsURL, "ws://example.com:8080")
	}
}

func TestNew_WithToken(t *testing.T) {
	p, err := New(map[string]any{
		"token": "my-secret-token",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	platform := p.(*Platform)
	if platform.token != "my-secret-token" {
		t.Errorf("token = %q, want %q", platform.token, "my-secret-token")
	}
}

func TestNew_WithAllowFrom(t *testing.T) {
	p, err := New(map[string]any{
		"allow_from": "user1,user2,*",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	platform := p.(*Platform)
	if platform.allowFrom != "user1,user2,*" {
		t.Errorf("allowFrom = %q, want %q", platform.allowFrom, "user1,user2,*")
	}
}

func TestNew_ShareSessionInChannel(t *testing.T) {
	p, err := New(map[string]any{
		"share_session_in_channel": true,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	platform := p.(*Platform)
	if !platform.shareSessionInChannel {
		t.Error("shareSessionInChannel = false, want true")
	}
}

// verify Platform implements core.Platform
var _ core.Platform = (*Platform)(nil)

// TestStart_FetchesSelfIDWithoutTimeout verifies that Start() completes
// promptly with selfID populated from the get_login_info OneBot API call.
// Regression for a bug where Start invoked callAPI BEFORE launching readLoop,
// so the API response had no consumer and callAPI always timed out after 15s
// — leaving selfID=0 and disabling the self-message filter in handleMessage.
func TestStart_FetchesSelfIDWithoutTimeout(t *testing.T) {
	const botUserID = 999999

	upgrader := websocket.Upgrader{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			var req map[string]any
			if err := json.Unmarshal(msg, &req); err != nil {
				continue
			}
			if req["action"] == "get_login_info" {
				echo, _ := req["echo"].(string)
				resp := map[string]any{
					"status":  "ok",
					"retcode": 0,
					"echo":    echo,
					"data":    map[string]any{"user_id": botUserID, "nickname": "TestBot"},
				}
				raw, _ := json.Marshal(resp)
				_ = c.WriteMessage(websocket.TextMessage, raw)
			}
		}
	}))
	defer ts.Close()

	p := &Platform{
		wsURL: "ws" + strings.TrimPrefix(ts.URL, "http"),
	}

	done := make(chan error, 1)
	go func() {
		done <- p.Start(func(core.Platform, *core.Message) {})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		_ = p.Stop()
		t.Fatal("Start did not complete within 5s; readLoop likely starts after callAPI, so get_login_info never gets a response")
	}
	defer p.Stop()

	if p.selfID != botUserID {
		t.Errorf("selfID = %d, want %d (self-message filter would be disabled)", p.selfID, botUserID)
	}
}

func TestNew_GroupReplyAllOptions(t *testing.T) {
	p, err := New(map[string]any{})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if got := p.(*Platform).groupReplyAll; got {
		t.Errorf("default groupReplyAll = true, want false (require mention)")
	}

	p, _ = New(map[string]any{"group_reply_all": true})
	if got := p.(*Platform).groupReplyAll; !got {
		t.Errorf("group_reply_all=true should set groupReplyAll, got false")
	}

	p, _ = New(map[string]any{"require_mention": false})
	if got := p.(*Platform).groupReplyAll; !got {
		t.Errorf("require_mention=false should set groupReplyAll, got false")
	}

	p, _ = New(map[string]any{"require_mention": true})
	if got := p.(*Platform).groupReplyAll; got {
		t.Errorf("require_mention=true should leave groupReplyAll false, got true")
	}
}

func TestIsSelfMentioned(t *testing.T) {
	p := &Platform{selfID: 123456}

	segmentMsg := func(segments ...map[string]any) map[string]any {
		anySegs := make([]any, 0, len(segments))
		for _, s := range segments {
			anySegs = append(anySegs, s)
		}
		return map[string]any{"message": anySegs}
	}
	at := func(qq string) map[string]any {
		return map[string]any{"type": "at", "data": map[string]any{"qq": qq}}
	}
	text := map[string]any{"type": "text", "data": map[string]any{"text": "hello"}}

	cases := []struct {
		name    string
		payload map[string]any
		want    bool
	}{
		{"at self as string", segmentMsg(at("123456")), true},
		{"at self as number", segmentMsg(map[string]any{"type": "at", "data": map[string]any{"qq": float64(123456)}}), true},
		{"at other user", segmentMsg(at("999")), false},
		{"at everyone", segmentMsg(at("all")), false},
		{"text only", segmentMsg(text), false},
		{"text plus at self", segmentMsg(text, at("123456")), true},
		{"raw CQ at self", map[string]any{"message": "[CQ:at,qq=123456] hi"}, true},
		{"raw CQ at other", map[string]any{"message": "[CQ:at,qq=999] hi"}, false},
		{"raw text only", map[string]any{"message": "hi"}, false},
	}
	for _, tc := range cases {
		if got := p.isSelfMentioned(tc.payload); got != tc.want {
			t.Errorf("%s: isSelfMentioned = %v, want %v", tc.name, got, tc.want)
		}
	}

	// Unknown selfID must be conservative: never treat as mentioned.
	p2 := &Platform{selfID: 0}
	if got := p2.isSelfMentioned(segmentMsg(at("123456"))); got {
		t.Errorf("selfID=0 should not report mentioned")
	}
}

func TestHandleMessage_GroupMentionFilter(t *testing.T) {
	var handled []string
	handler := func(p core.Platform, msg *core.Message) {
		handled = append(handled, msg.SessionKey+"|"+msg.Content)
	}

	newPlatform := func(groupReplyAll bool) *Platform {
		plat := &Platform{
			selfID:        123456,
			groupReplyAll: groupReplyAll,
			handler:       handler,
		}
		plat.groupNameCache.Store("888", "test group")
		return plat
	}

	groupMsg := func(withAt bool, msgID int64) map[string]any {
		segs := []any{}
		if withAt {
			segs = append(segs, map[string]any{"type": "at", "data": map[string]any{"qq": "123456"}})
		}
		segs = append(segs, map[string]any{"type": "text", "data": map[string]any{"text": "hello"}})
		return map[string]any{
			"post_type":    "message",
			"message_type": "group",
			"user_id":      float64(111),
			"group_id":     float64(888),
			"message_id":   float64(msgID),
			"sender":       map[string]any{"card": "", "nickname": "tester"},
			"message":      segs,
		}
	}

	// Default (require mention): group messages without @ are dropped.
	p := newPlatform(false)
	p.handleMessage(groupMsg(false, 1001))
	if len(handled) != 0 {
		t.Errorf("non-mentioned group message should be ignored, got %v", handled)
	}
	p.handleMessage(groupMsg(true, 1002))
	if len(handled) != 1 || handled[0] != "qq:888:111|hello" {
		t.Errorf("mentioned group message should be handled, got %v", handled)
	}

	// group_reply_all: every group message is handled.
	handled = nil
	p = newPlatform(true)
	p.handleMessage(groupMsg(false, 1003))
	if len(handled) != 1 {
		t.Errorf("group_reply_all should handle non-mentioned message, got %v", handled)
	}

	// Private messages are never subject to the mention filter.
	handled = nil
	p = newPlatform(false)
	p.handleMessage(map[string]any{
		"post_type":    "message",
		"message_type": "private",
		"user_id":      float64(111),
		"message_id":   float64(1002),
		"sender":       map[string]any{"card": "", "nickname": "tester"},
		"message":      []any{map[string]any{"type": "text", "data": map[string]any{"text": "hi"}}},
	})
	if len(handled) != 1 || handled[0] != "qq:111|hi" {
		t.Errorf("private message should bypass mention filter, got %v", handled)
	}
}
