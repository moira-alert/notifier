package bot

import "testing"

func TestRecepient(t *testing.T) {
	username := "test_username"
	r := recipient{username}

	destination := r.Destination()

	if destination != username {
		t.Errorf("Recipient should return uid as destination (%s!=%s)", username, destination)
	}
}
