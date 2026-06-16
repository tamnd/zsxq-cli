package zsxq

import (
	"testing"
)

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "zsxq" {
		t.Errorf("Scheme = %q, want zsxq", info.Scheme)
	}
	if len(info.Hosts) == 0 {
		t.Error("Hosts is empty")
	}
	if info.Identity.Binary != "zsxq" {
		t.Errorf("Identity.Binary = %q, want zsxq", info.Identity.Binary)
	}
}

func TestClassifyGroupURL(t *testing.T) {
	got, id, err := Domain{}.Classify("https://wx.zsxq.com/group/28855514824481")
	if err != nil {
		t.Fatalf("Classify group URL: %v", err)
	}
	if got != "group" {
		t.Errorf("type = %q, want group", got)
	}
	if id != "28855514824481" {
		t.Errorf("id = %q, want 28855514824481", id)
	}
}

func TestClassifyTopicURL(t *testing.T) {
	got, id, err := Domain{}.Classify("https://wx.zsxq.com/topic/18885512814151")
	if err != nil {
		t.Fatalf("Classify topic URL: %v", err)
	}
	if got != "topic" {
		t.Errorf("type = %q, want topic", got)
	}
	if id != "18885512814151" {
		t.Errorf("id = %q, want 18885512814151", id)
	}
}

func TestClassifyBareID(t *testing.T) {
	typ, id, err := Domain{}.Classify("28855514824481")
	if err != nil {
		t.Fatalf("Classify bare ID: %v", err)
	}
	if typ != "group" {
		t.Errorf("type = %q, want group", typ)
	}
	if id != "28855514824481" {
		t.Errorf("id = %q", id)
	}
}

func TestClassifyEmpty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("Classify empty string should return error")
	}
}

func TestLocateGroup(t *testing.T) {
	got, err := Domain{}.Locate("group", "28855514824481")
	want := WebBase + "/group/28855514824481"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateTopic(t *testing.T) {
	got, err := Domain{}.Locate("topic", "18885512814151")
	want := WebBase + "/topic/18885512814151"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("Locate with unknown type should return error")
	}
}
