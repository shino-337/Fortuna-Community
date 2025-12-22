#!/usr/bin/env python3
import bcrypt
import sys

password = "admin123"
hash_bytes = bcrypt.hashpw(password.encode('utf-8'), bcrypt.gensalt(rounds=12))
print(hash_bytes.decode('utf-8'))

