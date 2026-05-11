package postgres_test

import (
	"testing"

	"komorebi/internal/domain/environment"
	"komorebi/internal/infra/postgres"
)

// TestShadowRepo_ForRoute_EmptyTable verifies that ForRoute returns an empty
// (non-nil) slice and no error when the shadow_grid table has no matching rows.
// This covers the initial state before the PLATEAU pipeline has been run.
func TestShadowRepo_ForRoute_EmptyTable(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewShadowRepo(pool)

	// Use a short segment in central Tokyo (Chiyoda ward).
	cells, err := repo.ForRoute(environment.ShadowParams{
		RouteGeometryWKT: "LINESTRING(139.7500 35.6800, 139.7510 35.6810)",
		BufferM:          50,
		HourSlot:         12,
		Month:            7,
	})
	if err != nil {
		t.Fatalf("ForRoute: %v", err)
	}
	if cells == nil {
		t.Fatal("expected non-nil slice, got nil")
	}
	// Table may be empty — that is fine. Just confirm the type is correct.
	for _, c := range cells {
		if c.HourSlot != 12 {
			t.Errorf("expected HourSlot=12, got %d", c.HourSlot)
		}
		if c.Month != 7 {
			t.Errorf("expected Month=7, got %d", c.Month)
		}
		if c.ShadeCoverage < 0 || c.ShadeCoverage > 1 {
			t.Errorf("shade_coverage out of range: %f", c.ShadeCoverage)
		}
		if len(c.CellGeometry) < 3 {
			t.Errorf("cell geometry has fewer than 3 coords: %v", c.CellGeometry)
		}
	}
}

// TestShadowRepo_ForRoute_DefaultBuffer verifies that zero BufferM is treated
// as the 50 m default (no error, no panic).
func TestShadowRepo_ForRoute_DefaultBuffer(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewShadowRepo(pool)

	_, err := repo.ForRoute(environment.ShadowParams{
		RouteGeometryWKT: "LINESTRING(139.7500 35.6800, 139.7510 35.6810)",
		BufferM:          0, // should default to 50
		HourSlot:         8,
		Month:            1,
	})
	if err != nil {
		t.Fatalf("ForRoute with zero buffer: %v", err)
	}
}

// TestParsePolygonWKT_Unit exercises parsePolygonWKT parsing logic directly
// without a database connection.
func TestParsePolygonWKT_Unit(t *testing.T) {
	cases := []struct {
		name    string
		wkt     string
		want    [][2]float64
		wantErr bool
	}{
		{
			name: "standard POLYGON",
			wkt:  "POLYGON((139.75 35.68,139.76 35.68,139.76 35.69,139.75 35.69,139.75 35.68))",
			want: [][2]float64{
				{139.75, 35.68},
				{139.76, 35.68},
				{139.76, 35.69},
				{139.75, 35.69},
				{139.75, 35.68},
			},
		},
		{
			name: "POLYGON with space before paren",
			wkt:  "POLYGON ((139.75 35.68,139.76 35.68,139.76 35.69,139.75 35.69,139.75 35.68))",
			want: [][2]float64{
				{139.75, 35.68},
				{139.76, 35.68},
				{139.76, 35.69},
				{139.75, 35.69},
				{139.75, 35.68},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := postgres.ParsePolygonWKT(tc.wkt)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch: got %d want %d", len(got), len(tc.want))
			}
			for i, coord := range got {
				if coord[0] != tc.want[i][0] || coord[1] != tc.want[i][1] {
					t.Errorf("coord[%d]: got %v want %v", i, coord, tc.want[i])
				}
			}
		})
	}
}
