import pymortar
from pymortar.mortar_pb2 import FetchRequest

c = pymortar.Client('http://localhost:5001')

resp = c.qualify([
    "SELECT ?s ?equip WHERE { ?s a brick:Temperature_Sensor . ?s brick:isPointOf ?equip }",
    "SELECT ?vav WHERE { ?vav a brick:VAV . ?vav brick:feeds ?zone . ?zone a brick:HVAC_Zone }",
])


air_temp_sensor_query = """SELECT ?sensor ?equip WHERE {
    ?sensor    a     brick:Air_Temperature_Sensor .
    ?sensor    brick:isPointOf ?equip
}"""
air_temp_setpoint_query = """SELECT ?sp ?equip WHERE {
    ?sp    a     brick:Air_Temperature_Setpoint .
    ?sp    brick:isPointOf ?equip
}"""

qualify_resp = c.qualify([air_temp_sensor_query, air_temp_setpoint_query])
print(qualify_resp)
print("running on {0} sites".format(len(qualify_resp.sites)))
request = pymortar.FetchRequest(
        sites=qualify_resp.sites,
        views=[
            pymortar.View(
                name="airtemp_sensors",
                definition=air_temp_sensor_query,
            ),
            pymortar.View(
                name="airtemp_sps",
                definition=air_temp_setpoint_query,
            )
        ]
)
resp = c.fetch(request)
