#!/usr/bin/env python3
import runpy
from pathlib import Path
import unittest

check = runpy.run_path(str(Path(__file__).with_name("check-retired-routes.py")))["violations"]


class RetiredRouteGateTests(unittest.TestCase):
    def test_old_consumer_and_handler_rejected(self):
        for path, source in (
            ("core/internal/api/routes_policy.go", 'pol.GET("/rules/:id", GetRule(db))'),
            ("dashboard/api.ts", 'get("/monitoring/agents")'),
            ("agent/sender.go", 'Post("/api/v1/runtime/events")'),
            ("dashboard/api.ts", 'get("/api/v1/graph/shortest-path")'),
            ("core/internal/api/new_routes.go", 'g.GET("/query", ExecuteGraphQuery (db))'),
            ("core/internal/api/old.go", "func PostRuntimeEventsScoped(db DB) {}"),
        ):
            with self.subTest(path=path):
                self.assertTrue(check(path, source))

    def test_supported_contract_allowed(self):
        self.assertFalse(check("agent/sender.go", 'Post("/api/v2/runtime/events")'))
        self.assertFalse(check("core/internal/api/routes.go",
                               'GetPodNetworkTopDestinationsByUIDScoped(db)'))
        self.assertFalse(check("dashboard/api.ts", 'get("/api/v1/runtime/signals")'))


if __name__ == "__main__":
    unittest.main()
