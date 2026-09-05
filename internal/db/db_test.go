package db

import "testing"

func TestDatabaseName(t *testing.T) {
	cases := map[string]string{
		"mongodb://localhost:27017/SalesBotDB":               "SalesBotDB",
		"mongodb://mongoserver:27017/Other?authSource=admin": "Other",
		"mongodb+srv://user:pw@cluster.example.net/Prod":     "Prod",
		"mongodb://localhost:27017":                          DefaultDatabase,
		"mongodb://localhost:27017/":                         DefaultDatabase,
		"not a uri":                                          DefaultDatabase,
	}
	for uri, want := range cases {
		if got := DatabaseName(uri); got != want {
			t.Errorf("DatabaseName(%q) = %q, want %q", uri, got, want)
		}
	}
}
