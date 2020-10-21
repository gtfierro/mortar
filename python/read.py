import requests
import pyarrow as pa

# resp = requests.get('http://localhost:5001/query?id=1&id=2&start=2013-01-01T00:00:00Z')
# r = pa.ipc.open_stream(resp.content)
# df = r.read_pandas()
# print(df.head())

s = "SELECT ?x WHERE { ?x rdf:type brick:Temperature_Sensor }"
resp = requests.get(f'http://localhost:5001/query?source=ciee&sparql={s}&start=2018-01-01T00:00:00Z')
r = pa.ipc.open_stream(resp.content)
df = r.read_pandas()
print(len(df))
print(df.groupby('id').count())
print(df.head())
