#!/usr/bin/env python3
"""Mutation probes for the completed-path SQL/cache regression guard."""
import runpy
import unittest

check = runpy.run_path("scripts/verify/check-cluster-qualified-pod-routes.py")["strict_identity_errors"]

class IdentityGuardTests(unittest.TestCase):
    def test_rejects_uid_resource_uid_raw_sql_and_cache(self):
        for source in [
            'db.Where("uid = ?", uid).First(&pod)',
            'db.Where("resource_uid = ? AND insight_type = ?", uid, kind)',
            'db.Exec(`UPDATE insights SET status = 1 WHERE resource_uid = ?`)',
            'db.Where(`EXISTS (SELECT 1 FROM malware_matches mm WHERE mm.pod_uid = insights.resource_uid)`)',
            'm.items[podUID] = item',
        ]:
            with self.subTest(source=source):
                self.assertTrue(check(source))

    def test_accepts_scoped_sql_and_identity_key(self):
        self.assertFalse(check('db.Where("cluster_id = ? AND resource_uid = ?", cluster, uid)'))
        self.assertFalse(check('db.Exec(`UPDATE insights SET status = 1 WHERE cluster_id = ? AND resource_uid = ?`)'))
        self.assertFalse(check('m.items[key] = item'))

if __name__ == "__main__":
    unittest.main()
