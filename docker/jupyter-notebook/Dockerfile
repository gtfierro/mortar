FROM jupyter/scipy-notebook

USER root
COPY notebooks/requirements.txt .
RUN python -m pip install -r requirements.txt

WORKDIR work
COPY notebooks .

RUN chown -R $NB_UID:$NB_UID sample-data
RUN cd sample-data && unzip bldg1.zip
RUN chown -R $NB_UID:$NB_UID sample-data
RUN chmod -R 755 sample-data

USER $NB_UID
