#getent group omm >/dev/null || groupadd -r omm
#getent passwd omm >/dev/null || \
#useradd -r -g omm -d /home/omm  \
#    -c "Prometheus opengauss_exporter services" omm
#exit 0