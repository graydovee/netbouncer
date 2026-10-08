#!/usr/bin/env python3
"""Export/restore ONLY control data, never SQLite traffic pages or WAL.

Stop the old instance before the final export. Stream --export to an off-host
file, then restore that file to a NEW database path. The source is always read-only.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import sys
import time

TABLES = ("banned_ip_net_group", "banned_ip_net", "policies", "risk_events")


def export(source):
    conn = sqlite3.connect(Path(source).resolve().as_uri() + "?mode=ro", uri=True)
    conn.execute("PRAGMA query_only=ON")
    conn.execute("BEGIN")
    tables = []
    for name in TABLES:
        schema = conn.execute("SELECT sql FROM sqlite_master WHERE type='table' AND name=?", (name,)).fetchone()
        if schema is None:
            raise RuntimeError(f"missing control table {name}")
        columns = [r[1] for r in conn.execute(f'PRAGMA table_info("{name}")')]
        query = f'SELECT * FROM "{name}"'
        args = ()
        if name == "risk_events":
            query += " WHERE ts>=?"
            args = (int(time.time()) - 30 * 86400,)
        query += " ORDER BY id"
        rows = conn.execute(query, args).fetchall()
        indices = [r[0] for r in conn.execute("SELECT sql FROM sqlite_master WHERE type='index' AND tbl_name=? AND sql IS NOT NULL ORDER BY name", (name,))]
        tables.append({"name": name, "schema": schema[0], "columns": columns, "rows": rows, "indices": indices})
    conn.rollback()
    conn.close()
    payload = {"format": "netbouncer-control-v1", "tables": tables}
    payload["sha256"] = hashlib.sha256(json.dumps(tables, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    return payload


def restore(payload, destination):
    if payload.get("format") != "netbouncer-control-v1":
        raise RuntimeError("unsupported backup format")
    tables = payload["tables"]
    digest = hashlib.sha256(json.dumps(tables, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    if digest != payload["sha256"] or tuple(t["name"] for t in tables) != TABLES:
        raise RuntimeError("backup checksum/table mismatch")
    # Exclusive creation prevents accidentally overwriting a live database.
    fd = os.open(destination, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    os.close(fd)
    try:
        conn = sqlite3.connect(destination)
        conn.execute("BEGIN")
        for table in tables:
            conn.execute(table["schema"])
            names = ",".join('"' + c.replace('"', '""') + '"' for c in table["columns"])
            marks = ",".join("?" for _ in table["columns"])
            conn.executemany(f'INSERT INTO "{table["name"]}"({names}) VALUES({marks})', table["rows"])
            for index in table["indices"]:
                conn.execute(index)
            actual = conn.execute(f'SELECT * FROM "{table["name"]}" ORDER BY id').fetchall()
            if actual != [tuple(row) for row in table["rows"]]:
                raise RuntimeError(f"restore mismatch for {table['name']}")
        if conn.execute("PRAGMA integrity_check").fetchone()[0] != "ok":
            raise RuntimeError("restored control integrity check failed")
        conn.commit()
        counts = {t["name"]: len(t["rows"]) for t in tables}
        conn.close()
        return {"sha256": digest, "counts": counts, "database_bytes": Path(destination).stat().st_size}
    except BaseException:
        try:
            conn.close()
        except UnboundLocalError:
            pass
        Path(destination).unlink(missing_ok=True)
        raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("--export", metavar="READ_ONLY_DB")
    group.add_argument("--restore", metavar="BACKUP_JSON")
    parser.add_argument("--destination")
    args = parser.parse_args()
    if args.export:
        json.dump(export(args.export), sys.stdout, separators=(",", ":"))
    else:
        if not args.destination:
            parser.error("--destination is required for restore")
        print(json.dumps(restore(json.loads(Path(args.restore).read_text()), args.destination), ensure_ascii=False))


if __name__ == "__main__":
    main()
