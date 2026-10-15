#!/usr/bin/python3
from hdbcli import dbapi
import hashlib
import argparse

parser = argparse.ArgumentParser(description="bstring test script")
parser.add_argument("address", help="address")
parser.add_argument('port', type=int, help='port: 3xxxx')
parser.add_argument('user', help='user')
parser.add_argument('password', help='password')
args = parser.parse_args()

try:
    conn = dbapi.connect(address=args.address, port=args.port,user=args.user, password=args.password)
    try:
        cursor = conn.cursor()
        try:
            hash = hashlib.sha256()
            hash.update(b"TEST")
            cursor.execute("SELECT 'FOOBAR' FROM DUMMY WHERE HASH_SHA256('TEST') = :id", {"id": hash.digest()})
        except Exception as err:
            print("error: {}".format(err))
        finally:
            cursor.close()
    except Exceptiopn as err:
        print("error: {}".format(err))
    finally:
        conn.close()
except Exception as err:
    print("error: {}".format(err))
