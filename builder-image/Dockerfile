FROM centos:8.1.1911
RUN yum -y install nodejs java-1.8.0-openjdk-devel procps git wget unzip sudo ca-certificates maven \
    && yum clean all \
    && rm -rf /var/cache/yum

ARG BUILDAH_RPM=https://cbs.centos.org/kojifiles/packages/buildah/1.11.4/4.el7/x86_64/buildah-1.11.4-4.el7.x86_64.rpm
ARG SLIRP=https://cbs.centos.org/kojifiles/packages/slirp4netns/0.3.0/2.git4992082.el7/x86_64/slirp4netns-0.3.0-2.git4992082.el7.x86_64.rpm
ARG LIBSEC=https://cbs.centos.org/kojifiles/packages/libseccomp/2.4.1/0.el7/x86_64/libseccomp-2.4.1-0.el7.x86_64.rpm

RUN curl -f -o buildah.rpm $BUILDAH_RPM \
    && curl -f -o slirp4netns.rpm $SLIRP \
    && curl -f -o libseccomp.rpm $LIBSEC \
    && yum -y install zip unzip ca-certificates openssl slirp4netns.rpm libseccomp.rpm buildah.rpm  \
    && yum clean all \
    && rm -rf /var/cache/yum /buildah.rpm /slirp4netns.rpm /libseccomp.rpm

COPY project /root/project

COPY container-resources /root

RUN cd /root/project && mvn package

CMD /root/entrypoint.sh
