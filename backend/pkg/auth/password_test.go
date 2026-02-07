package auth

import "testing"

func TestHashPasswordAndCheckPasswordBcrypt(t *testing.T) {
	hash, err := HashPassword("s3cret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if hash == "" {
		t.Fatalf("expected non-empty hash")
	}
	if !CheckPassword("s3cret", hash) {
		t.Fatalf("expected bcrypt password check to pass")
	}
	if CheckPassword("wrong", hash) {
		t.Fatalf("expected bcrypt password check to fail")
	}
}

func TestValidatePassword(t *testing.T) {
	valid := "Str0ng#Password!"
	if err := ValidatePassword(valid); err != nil {
		t.Fatalf("expected valid password, got: %v", err)
	}
	if err := ValidatePassword("short1!A"); err == nil {
		t.Fatalf("expected short password to fail")
	}
	if err := ValidatePassword("alllowercase123!"); err == nil {
		t.Fatalf("expected missing uppercase to fail")
	}
	if err := ValidatePassword("ALLUPPERCASE123!"); err == nil {
		t.Fatalf("expected missing lowercase to fail")
	}
	if err := ValidatePassword("NoDigitsHere!!!"); err == nil {
		t.Fatalf("expected missing digits to fail")
	}
	if err := ValidatePassword("NoSpecials1234"); err == nil {
		t.Fatalf("expected missing special chars to fail")
	}
}
