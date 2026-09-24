#!/usr/bin/env python3
"""Drive a Hookdeck MCP server over stdio for manual QA.

The MCP surfaces cannot be exercised with plain CLI invocations, and testing
them by hand through an editor is slow and hard to repeat. This speaks the
protocol directly so a QA pass can call any tool and see the raw result.

    # what does the server advertise, and in which mode?
    mcp-call.py --config "$HD_CONFIG" --server gateway --list
    mcp-call.py --config "$HD_CONFIG" --server gateway --allow-write --list

    # call a tool
    mcp-call.py --config "$HD_CONFIG" --server outpost --allow-write \
        --tool outpost_destinations \
        --args '{"action":"create","tenant_id":"t1","type":"webhook",
                 "config":{"url":"https://example.com/hook"}}'

Exits non-zero if the tool reports an error, so it can gate a script.
"""

import argparse
import json
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True, help="isolated config from hd_testenv")
    parser.add_argument("--server", default="gateway", choices=["gateway", "outpost"])
    parser.add_argument("--allow-write", action="store_true")
    parser.add_argument("--tool")
    parser.add_argument("--args", default="{}", help="tool arguments as JSON")
    parser.add_argument("--list", action="store_true", help="list tools instead of calling one")
    parser.add_argument("--binary", default="./hookdeck", help="built CLI to run")
    opts = parser.parse_args()

    if not opts.list and not opts.tool:
        parser.error("one of --tool or --list is required")

    cmd = [opts.binary, "--hookdeck-config", opts.config, opts.server, "mcp"]
    if opts.allow_write:
        cmd.append("--allow-write")

    proc = subprocess.Popen(
        cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
        stderr=subprocess.PIPE, text=True,
    )

    next_id = [0]

    def rpc(method, params=None):
        next_id[0] += 1
        request_id = next_id[0]
        message = {"jsonrpc": "2.0", "id": request_id, "method": method}
        if params is not None:
            message["params"] = params
        proc.stdin.write(json.dumps(message) + "\n")
        proc.stdin.flush()
        while True:
            line = proc.stdout.readline()
            if not line:
                sys.stderr.write(proc.stderr.read())
                raise SystemExit(f"server closed the connection during {method}")
            try:
                parsed = json.loads(line)
            except json.JSONDecodeError:
                continue  # log line on stdout, not a protocol message
            if parsed.get("id") == request_id:
                return parsed

    try:
        rpc("initialize", {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "hookdeck-manual-qa", "version": "1"},
        })
        proc.stdin.write(json.dumps({"jsonrpc": "2.0", "method": "notifications/initialized"}) + "\n")
        proc.stdin.flush()

        if opts.list:
            tools = rpc("tools/list")["result"]["tools"]
            for tool in sorted(tools, key=lambda t: t["name"]):
                actions = tool.get("inputSchema", {}).get("properties", {}).get("action", {})
                available = actions.get("enum", [])
                print(f"{tool['name']:<28} {', '.join(available) if available else '(no action enum)'}")
            print(f"\n{len(tools)} tools", file=sys.stderr)
            return 0

        response = rpc("tools/call", {"name": opts.tool, "arguments": json.loads(opts.args)})
        if "error" in response:
            print(json.dumps(response["error"], indent=2))
            return 1

        result = response["result"]
        for block in result.get("content", []):
            text = block.get("text", "")
            try:
                print(json.dumps(json.loads(text), indent=2))
            except json.JSONDecodeError:
                print(text)
        # isError means the tool reported failure to the model. A QA script that
        # ignored it would record a failed call as a passing one.
        if result.get("isError"):
            print("\n[isError: the tool reported this as a failure]", file=sys.stderr)
            return 1
        return 0
    finally:
        proc.terminate()


if __name__ == "__main__":
    sys.exit(main())
