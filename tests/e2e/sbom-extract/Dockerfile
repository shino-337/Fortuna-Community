FROM centos:7

# Không chạy yum update để giữ nguyên các package vulnerable
# Bạn có thể thêm package nào cũng được, ví dụ:
RUN yum install -y \
    openssl \
    curl \
    bash \
    glibc \
    libxml2 \
    && yum clean all

# Hoặc để giữ pod chạy
CMD ["sleep", "infinity"]
