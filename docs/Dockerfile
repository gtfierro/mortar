FROM jupyter/scipy-notebook
# Set up the user environment

COPY requirements.txt requirements.txt
RUN ([ -f requirements.txt ] \
    && python -m pip install --no-cache-dir -r requirements.txt)

USER root
RUN jupyter sparqlkernel install

ENV NB_USER jovyan
ENV NB_UID 1000
ENV HOME /home/$NB_USER

COPY . $HOME
RUN chown -R $NB_UID $HOME

USER $NB_USER

# Launch the notebook server
WORKDIR $HOME
CMD ["jupyter", "notebook", "--ip", "0.0.0.0"]
