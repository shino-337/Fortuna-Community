package resourceidentity

import "testing"

func TestIdentityRequiresClusterAndUID(t *testing.T) {
	for _, tc := range []struct {
		name      string
		clusterID string
		uid       string
	}{
		{name: "missing cluster", uid: "pod-1"},
		{name: "missing uid", clusterID: "cluster-a"},
		{name: "cluster outer whitespace", clusterID: " cluster-a", uid: "pod-1"},
		{name: "uid outer whitespace", clusterID: "cluster-a", uid: "pod-1 "},
		{name: "cluster control", clusterID: "cluster-a\n", uid: "pod-1"},
		{name: "uid control", clusterID: "cluster-a", uid: "pod\x001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.clusterID, tc.uid); err == nil {
				t.Fatalf("New(%q, %q) succeeded, want rejection", tc.clusterID, tc.uid)
			}
		})
	}

	id, err := New("cluster-a", "pod-1")
	if err != nil {
		t.Fatalf("valid identity rejected: %v", err)
	}
	if id.ClusterID != "cluster-a" || id.ResourceUID != "pod-1" {
		t.Fatalf("unexpected identity: %#v", id)
	}
}

func TestIdentitySeparatesDuplicateUIDAcrossClusters(t *testing.T) {
	a, err := New("cluster-a", "same-pod-uid")
	if err != nil {
		t.Fatal(err)
	}
	b, err := New("cluster-b", "same-pod-uid")
	if err != nil {
		t.Fatal(err)
	}
	if a.Equal(b) {
		t.Fatal("same pod UID in different clusters must not be the same identity")
	}
	ak, err := a.Key()
	if err != nil {
		t.Fatal(err)
	}
	bk, err := b.Key()
	if err != nil {
		t.Fatal(err)
	}
	if ak == bk {
		t.Fatalf("identity keys collided: %q", ak)
	}

	// Length-prefixing prevents ambiguous delimiter concatenation.
	x, _ := New("a:b", "c")
	y, _ := New("a", "b:c")
	xk, _ := x.Key()
	yk, _ := y.Key()
	if xk == yk {
		t.Fatalf("delimiter-shaped identities collided: %q", xk)
	}
}
