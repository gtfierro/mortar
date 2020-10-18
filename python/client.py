import requests
from datetime import datetime, timedelta
import random


def now():
    return datetime.now().strftime("%Y-%m-%dT%H:%M:%SZ%Z")


def generate_n(n):
    st = datetime.now()
    return [
        [(st + timedelta(seconds=x)).strftime("%Y-%m-%dT%H:%M:%SZ%Z"),
         random.randint(0, 100)]
        for x in range(n)
    ]


# stream1 = {
#     "SourceName": "testsource1",
#     "Units": "degF",
#     "Name": "stream1"
# }
#
# N = 50000
#
# resp = requests.post("http://localhost:5001/register_stream", json=stream1)
# if not resp.ok:
#     print(resp.content)
#
# readings = generate_n(N)
# ds = {
#     "SourceName": "testsource1",
#     "Name": "stream1",
#     "Readings": readings
# }
#
# resp = requests.post('http://localhost:5001/insert_bulk', json=ds)
# if not resp.ok:
#     print(resp.content)


with open('Brick.ttl', 'rb') as f:
    resp = requests.post('http://localhost:5001/insert_triple_file?source=test&origin=brick', data=f.read())
    if not resp.ok:
        print(resp.content)

# with open('ebu3b.ttl', 'rb') as f:
#     resp = requests.post('http://localhost:5001/insert_triple_file?source=test&origin=bldg', data=f.read())
#     if not resp.ok:
#         print(resp.content)

with open('ciee.ttl', 'rb') as f:
    resp = requests.post('http://localhost:5001/insert_triple_file?source=test&origin=bldg', data=f.read())
    if not resp.ok:
        print(resp.content)
# csv file
# f1 = "../mortar-timescale/data/ciee/out1.csv"
# with open(f1, 'rb') as f:
#     resp = requests.post('http://localhost:5001/insert_streaming?source=testsource1&name=stream1', data=f)
#     if not resp.ok:
#         print(resp.content)
