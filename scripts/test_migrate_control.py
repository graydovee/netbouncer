import json
from pathlib import Path
import sqlite3
import tempfile
import time
import unittest

from migrate_control import export, restore


class ControlMigrationTest(unittest.TestCase):
    def test_restore_preserves_values_and_excludes_legacy_traffic(self):
        with tempfile.TemporaryDirectory() as directory:
            old = Path(directory) / 'old.sqlite'
            new = Path(directory) / 'new.sqlite'
            conn = sqlite3.connect(old)
            for name in ('banned_ip_net_group', 'banned_ip_net', 'policies'):
                conn.execute(f'CREATE TABLE {name}(id INTEGER PRIMARY KEY, value TEXT, optional TEXT)')
                conn.execute(f'INSERT INTO {name} VALUES(1,?,NULL)', ('quoted " value',))
            conn.execute('CREATE TABLE risk_events(id INTEGER PRIMARY KEY,ts INTEGER,value TEXT)')
            now = int(time.time())
            conn.executemany('INSERT INTO risk_events VALUES(?,?,?)', [(1, now, 'valid'), (2, now-31*86400, 'expired')])
            conn.execute('CREATE TABLE traffic_samples(id INTEGER PRIMARY KEY,payload TEXT)')
            conn.execute("INSERT INTO traffic_samples VALUES(1,'legacy')")
            conn.commit()
            conn.close()
            payload = json.loads(json.dumps(export(old)))
            result = restore(payload, new)
            self.assertEqual(result['counts']['risk_events'], 1)
            restored = sqlite3.connect(new)
            self.assertEqual(restored.execute('SELECT * FROM banned_ip_net').fetchall(), [(1, 'quoted " value', None)])
            self.assertIsNone(restored.execute("SELECT name FROM sqlite_master WHERE name='traffic_samples'").fetchone())
            restored.close()
            self.assertEqual(sqlite3.connect(old).execute('SELECT count(*) FROM risk_events').fetchone()[0], 2)
            with self.assertRaises(FileExistsError):
                restore(payload, new)
            payload['tables'][0]['rows'][0][1] = 'changed'
            with self.assertRaisesRegex(RuntimeError, 'checksum'):
                restore(payload, Path(directory) / 'tampered.sqlite')


if __name__ == '__main__':
    unittest.main()
