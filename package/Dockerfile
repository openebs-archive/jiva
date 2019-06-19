FROM openebs/jiva:base-xenial-20190515
#RUN apt-get update && apt-get install -y kmod curl nfs-common fuse \
#        libibverbs1 librdmacm1 libconfig-general-perl libaio1 \
#        && rm -rf /var/lib/apt/lists/*
RUN apt-get update && apt-get install -y curl \
    && rm -rf /var/lib/apt/lists/*

COPY longhorn jivactl launch copy-binary launch-with-vm-backing-file launch-simple-jiva /usr/local/bin/

VOLUME /usr/local/bin
CMD ["longhorn"]
