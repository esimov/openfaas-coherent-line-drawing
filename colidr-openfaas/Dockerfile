FROM denismakogon/ffmpeg-alpine:4.0-golang-1.11 as builder

LABEL maintainer="Denis Makogon. mail: lildee1991@gmail.com"

ENV OPENCV_VERSION=3.4.2
ENV BUILD="ca-certificates \
         git \
         build-base \
         musl-dev \
         alpine-sdk \
         make \
         gcc \
         g++ \
         libc-dev \
         linux-headers \
         libjpeg-turbo \
         libpng \
         libwebp \
         libwebp-dev \
         tiff \
         libavc1394 \
         jasper-libs \
         openblas \
         libgphoto2 \
         gstreamer \
         gst-plugins-base"

ENV DEV="clang clang-dev cmake pkgconf \
         openblas-dev gstreamer-dev gst-plugins-base-dev \
         libgphoto2-dev libjpeg-turbo-dev libpng-dev \
         tiff-dev jasper-dev libavc1394-dev"


RUN apk update && \
    apk add --no-cache ${BUILD} ${DEV}

RUN mkdir /tmp/opencv && \
    cd /tmp/opencv && \
    wget -O opencv.zip https://github.com/opencv/opencv/archive/${OPENCV_VERSION}.zip && \
    unzip opencv.zip && \
    wget -O opencv_contrib.zip https://github.com/opencv/opencv_contrib/archive/${OPENCV_VERSION}.zip && \
    unzip opencv_contrib.zip && \
    mkdir /tmp/opencv/opencv-${OPENCV_VERSION}/build && cd /tmp/opencv/opencv-${OPENCV_VERSION}/build && \
    cmake \
    -D CMAKE_BUILD_TYPE=RELEASE \
    -D CMAKE_INSTALL_PREFIX=/usr/local \
    -D OPENCV_EXTRA_MODULES_PATH=/tmp/opencv/opencv_contrib-${OPENCV_VERSION}/modules \
    -D WITH_FFMPEG=YES \
    -D INSTALL_C_EXAMPLES=NO \
    -D INSTALL_PYTHON_EXAMPLES=NO \
    -D BUILD_ANDROID_EXAMPLES=NO \
    -D BUILD_DOCS=NO \
    -D BUILD_TESTS=NO \
    -D BUILD_PERF_TESTS=NO \
    -D BUILD_EXAMPLES=NO \
    -D BUILD_opencv_java=NO \
    -D BUILD_opencv_python=NO \
    -D BUILD_opencv_python2=NO \
    -D BUILD_opencv_python3=NO \
    -D OPENCV_GENERATE_PKGCONFIG=YES .. && \
    make -j4 && \
    make install && \
    cd && rm -rf /tmp/opencv

RUN apk del ${DEV_DEPS} && \
    rm -rf /var/cache/apk/*

ENV PKG_CONFIG_PATH /usr/local/lib64/pkgconfig
ENV LD_LIBRARY_PATH /usr/local/lib64
ENV CGO_CPPFLAGS -I/usr/local/include
ENV CGO_CXXFLAGS "--std=c++1z"
ENV CGO_LDFLAGS "-L/usr/local/lib -lopencv_core -lopencv_face -lopencv_videoio -lopencv_imgproc -lopencv_highgui -lopencv_imgcodecs -lopencv_objdetect -lopencv_features2d -lopencv_video -lopencv_dnn -lopencv_xfeatures2d -lopencv_plot -lopencv_tracking"

# Add the watchdog
RUN apk upgrade --no-cache && apk --no-cache add curl \
    && echo "Pulling watchdog binary from Github." \
    && curl -sSL https://github.com/openfaas/faas/releases/download/0.8.10/fwatchdog > /usr/bin/fwatchdog \
    && chmod +x /usr/bin/fwatchdog \
    && apk del curl --no-cache

WORKDIR /go/src/handler
COPY . .

# Run a gofmt and exclude all vendored code.
RUN test -z "$(gofmt -l $(find . -type f -name '*.go' -not -path "./vendor/*" -not -path "./function/vendor/*"))" || { echo "Run \"gofmt -s -w\" on your Golang code"; exit 1; }

RUN go build --ldflags "-s -w" -a -installsuffix cgo -o handler .

FROM denismakogon/gocv-alpine:3.4.2-runtime

RUN echo -e '@edgunity http://nl.alpinelinux.org/alpine/edge/community\n\
@edge http://nl.alpinelinux.org/alpine/edge/main\n\
@testing http://nl.alpinelinux.org/alpine/edge/testing\n\
@community http://dl-cdn.alpinelinux.org/alpine/edge/community'\
  >> /etc/apk/repositories

RUN apk --no-cache add ca-certificates
# libstdc++ libjpeg libtbb@testing libpng jasper-libs tiff openblas libwebp

# Add non root user
RUN addgroup -S app && adduser -S -g app app
RUN mkdir -p /home/app

# Copy OpenCV Libs
COPY --from=builder /usr/local/lib /usr/local/lib
COPY --from=builder /usr/local/include /usr/local/include

WORKDIR /home/app

COPY --from=builder /go/src/handler/function/  .
COPY --from=builder /go/src/handler/handler    .
COPY --from=builder /usr/bin/fwatchdog         .

RUN chown -R app /home/app

USER app

ENV fprocess="./handler"

CMD ["./fwatchdog"]