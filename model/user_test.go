package model

import "testing"

func TestUserPasswordRoundTrip(t *testing.T) {
	var user User
	if err := user.SetPassword("secret1"); err != nil {
		t.Fatal(err)
	}
	if user.PasswordDigest == "" || user.PasswordDigest == "secret1" {
		t.Fatalf("digest = %q", user.PasswordDigest)
	}
	if !user.CheckPassword("secret1") {
		t.Fatal("expected the password to match")
	}
	if user.CheckPassword("other") {
		t.Fatal("expected a different password to fail")
	}
	if (*User)(nil).CheckPassword("secret1") {
		t.Fatal("nil user should not match")
	}

	digest, err := CreateUserPassword("secret1")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPasswordDigest(digest, "secret1") || CheckPasswordDigest(digest, "nope") {
		t.Fatal("CreateUserPassword digest did not verify")
	}
}

func TestUserIDPattern(t *testing.T) {
	for _, id := range []string{"abcd", "a.bc", "a_b-c", "a123"} {
		if !UserIDPattern.MatchString(id) {
			t.Fatalf("%q should be a valid username", id)
		}
	}
	for _, id := range []string{"", "ab", "Abc", "1abc", "a b", "a@b"} {
		if UserIDPattern.MatchString(id) {
			t.Fatalf("%q should be rejected", id)
		}
	}
}
