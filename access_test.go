package bunker

import (
	"testing"

	"github.com/yankeguo/bunker/model"
)

func TestExpandGrantedItems(t *testing.T) {
	grants := []*model.Grant{
		{ServerUser: "*", ServerID: "web*"},
		{ServerUser: "deploy", ServerID: "web-1"},
		{ServerUser: "root", ServerID: "db"},
		nil,
	}
	servers := []*model.Server{
		{ID: "web-1"},
		{ID: "web-2"},
		{ID: "db"},
		nil,
	}

	items := expandGrantedItems(grants, servers)
	if len(items) != 4 {
		t.Fatalf("got %d items, want 4: %#v", len(items), items)
	}

	want := []grantedItem{
		{ServerUser: "root", ServerID: "db"},
		{ServerUser: "*", ServerID: "web-1"},
		{ServerUser: "deploy", ServerID: "web-1"},
		{ServerUser: "*", ServerID: "web-2"},
	}
	for i, item := range want {
		if items[i] != item {
			t.Fatalf("item %d = %#v, want %#v", i, items[i], item)
		}
	}
}

func TestExpandGrantedItemsCollapsesDuplicateUsers(t *testing.T) {
	items := expandGrantedItems(
		[]*model.Grant{
			{ServerUser: "root", ServerID: "web*"},
			{ServerUser: "root", ServerID: "*"},
		},
		[]*model.Server{{ID: "web-1"}},
	)
	if len(items) != 1 || items[0] != (grantedItem{ServerUser: "root", ServerID: "web-1"}) {
		t.Fatalf("items = %#v", items)
	}
}

func TestExpandGrantedItemsEmptyIsNonNil(t *testing.T) {
	items := expandGrantedItems(nil, nil)
	if items == nil || len(items) != 0 {
		t.Fatalf("items = %#v", items)
	}
}

func TestGrantMatches(t *testing.T) {
	grant := &model.Grant{ServerUser: "web*", ServerID: "prod-*"}
	if !grantMatches(grant, "webapp", "prod-1") {
		t.Fatal("expected wildcard grant to match")
	}
	if !grantMatches(grant, "WEBAPP", "Prod-1") {
		t.Fatal("expected case-insensitive match")
	}
	if grantMatches(grant, "root", "prod-1") {
		t.Fatal("server user wildcard should not match root")
	}
	if grantMatches(nil, "webapp", "prod-1") {
		t.Fatal("nil grant should not match")
	}
	single := &model.Grant{ServerUser: "web-?", ServerID: "db"}
	if !grantMatches(single, "web-1", "db") {
		t.Fatal("expected single-character wildcard to match")
	}
	if grantMatches(single, "web-12", "db") {
		t.Fatal("single-character wildcard matched more than one character")
	}
}
