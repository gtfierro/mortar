import toml
from functools import lru_cache
# would use cached_property but we need to be compliant down to python 3.7

class Application:
    def __init__(self, filename, client):
        self.spec = toml.load(open(filename))
        self.queries = self.spec["queries"]
        self.name = self.spec["name"]
        self.client = client

    @property
    @lru_cache(maxsize=0)
    def valid_sites(self):
        return self.refresh_valid_sites()

    def refresh_valid_sites(self):
        df = self.client.qualify(self.queries).df
        sites = list(df[df.all(axis=1)].index)
        return sites
