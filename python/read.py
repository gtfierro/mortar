import requests
import pyarrow as pa

resp = requests.get('http://localhost:5002/query?id=1&id=2&start=2013-01-01T00:00:00Z')
r = pa.ipc.open_stream(resp.content)
df = r.read_pandas()
print(df.head())
