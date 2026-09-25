import re
path = r"E:\渗透\新渠道\atria2api\internal\server\management.go"
with open(path, "rb") as f:
    data = f.read()
# Match: acct.Formula = "..." where the string contains any non-ASCII byte
pattern = rb'acct\.Formula = "[^"]*[\x80-\xff][^"]*"'
replacement = rb'acct.Formula = "\\u672a\\u7f13\\u5b58\\u8f93\\u5165 \\u00d7 20% + \\u8f93\\u51fa\\uff0c\\u7f13\\u5b58\\u8f93\\u5165\\u514d\\u8d39"'
new_data, n = re.subn(pattern, replacement, data)
print(f"replaced {n} occurrence(s)")
with open(path, "wb") as f:
    f.write(new_data)
# verify
with open(path, "rb") as f:
    d = f.read()
for i, line in enumerate(d.split(b"\n"), 1):
    if b"acct.Formula" in line and b"\\u" in line:
        print(f"line {i}: {line.decode('ascii').strip()}")
