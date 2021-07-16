FROM jupyter/scipy-notebook

USER root
COPY notebooks/requirements.txt .
RUN python -m pip install --ignore-installed -r requirements.txt


WORKDIR work
COPY docker/jupyter-notebook/trust-notebooks.sh .
COPY notebooks .

RUN chown -R $NB_UID:$NB_UID sample-data
RUN cd sample-data && unzip bldg1.zip
RUN chown -R $NB_UID:$NB_UID sample-data
RUN chmod -R 755 sample-data

ENV JUPYTER_ENABLE_LAB=yes
USER $NB_UID

RUN bash trust-notebooks.sh
