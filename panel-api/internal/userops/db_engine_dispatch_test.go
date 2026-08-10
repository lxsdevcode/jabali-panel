package userops

// The delete cascade sends one agent command per database and per database
// login. Both the command name and the payload key differ by engine, and
// getting either wrong is silent: `db.drop` / `db_user.drop` reach MariaDB,
// whose DROP ... IF EXISTS succeeds on a name that was never there. The
// cascade's failure guard therefore never fires, the user row is deleted,
// its metadata rows CASCADE away — and the real Postgres database or role
// survives on the host with nothing left to name it (GH #1013).
//
// That makes this dispatch the whole fix, so it is pinned directly.

import "testing"

func TestDBUserDropCmd_DispatchesOnEngine(t *testing.T) {
	if got := dbUserDropCmd("postgres"); got != "db.postgres.drop_role" {
		t.Errorf("postgres login drop = %q, want db.postgres.drop_role — db_user.drop reaches MariaDB and no-ops", got)
	}
	for _, engine := range []string{"mariadb", "", "mysql"} {
		if got := dbUserDropCmd(engine); got != "db_user.drop" {
			t.Errorf("engine %q login drop = %q, want db_user.drop", engine, got)
		}
	}
}

// A Postgres drop_role reads "role"; sending the MariaDB "db_user_name"
// shape leaves the role name empty, so the command is a no-op even when
// the command NAME is right.
func TestDBUserDropParams_KeyDiffersByEngine(t *testing.T) {
	pg := dbUserDropParams("postgres", "alice_app")
	if pg["role"] != "alice_app" {
		t.Errorf("postgres params = %#v, want role=alice_app", pg)
	}
	if _, ok := pg["db_user_name"]; ok {
		t.Error("postgres params must not carry db_user_name — drop_role ignores it")
	}

	my := dbUserDropParams("mariadb", "alice_app")
	if my["db_user_name"] != "alice_app" {
		t.Errorf("mariadb params = %#v, want db_user_name=alice_app", my)
	}
	if _, ok := my["role"]; ok {
		t.Error("mariadb params must not carry role")
	}
}

func TestDBDropCmd_DispatchesOnEngine(t *testing.T) {
	if got := dbDropCmd("postgres"); got != "db.postgres.drop_db" {
		t.Errorf("postgres database drop = %q, want db.postgres.drop_db — db.drop reaches "+
			"MariaDB, whose DROP DATABASE IF EXISTS succeeds on a name that was never there, "+
			"so the cascade's failure guard never fires", got)
	}
	for _, engine := range []string{"mariadb", "", "mysql"} {
		if got := dbDropCmd(engine); got != "db.drop" {
			t.Errorf("engine %q database drop = %q, want db.drop", engine, got)
		}
	}
}
