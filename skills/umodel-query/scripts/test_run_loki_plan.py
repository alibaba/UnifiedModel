import io
import json
import unittest

from run_loki_plan import execute


class LokiPlanExecutionTest(unittest.TestCase):
    def test_logs_use_query_range_and_merge_sorted_entries(self):
        calls = []

        def opener(req):
            calls.append(req)
            index = len(calls)
            result = {
                "status": "success",
                "data": {"resultType": "streams", "result": [{
                    "stream": {"host": "node-" + str(index)},
                    "values": [[str(index), "line " + str(index)]],
                }]},
            }
            return io.BytesIO(json.dumps(result).encode())

        plan = {
            "mode": "plan", "operation": "get_logs",
            "time_range": {"from": "2026-11-03T10:00:00+08:00", "to": "2026-11-03T11:00:00+08:00"},
            "query": {"dialect": "loki_logql", "endpoint": "http://localhost:3100", "api_prefix": "/loki/api/v1",
                      "query_type": "range", "queries": [{"logql": '{host="node-1"}'}, {"logql": '{host="node-2"}'}],
                      "size": 1, "direction": "backward"},
        }
        result = execute(plan, opener)
        self.assertEqual(result["entries"][0]["line"], "line 2")
        self.assertEqual(len(result["entries"]), 1)
        self.assertEqual(len(calls), 2)
        self.assertIn("/loki/api/v1/query_range?", calls[0].full_url)
        self.assertIn("direction=backward", calls[0].full_url)

    def test_instant_metric_uses_query_endpoint(self):
        calls = []

        def opener(req):
            calls.append(req)
            return io.BytesIO(json.dumps({"status": "success", "data": {
                "resultType": "vector", "result": [{"metric": {}, "value": [1, "2"]}],
            }}).encode())

        plan = {"mode": "plan", "operation": "get_metrics", "time_range": {"to": "2026-11-03T11:00:00+08:00"},
                "query": {"dialect": "loki_logql", "endpoint": "http://localhost:3100", "query_type": "instant",
                          "queries": [{"name": "errors", "logql": 'count_over_time({job="api"}[5m])'}]}}
        result = execute(plan, opener)
        self.assertEqual(result["result_type"], "vector")
        self.assertIn("/loki/api/v1/query?", calls[0].full_url)
        self.assertEqual(result["queries"][0]["name"], "errors")


if __name__ == "__main__":
    unittest.main()
