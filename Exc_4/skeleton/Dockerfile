  # todo
FROM golang:1.27 AS builder

WORKDIR /app
COPY . /app
RUN sed -i 's/\r$//' scripts/build-application.sh
RUN sh scripts/build-application.sh

FROM alpine AS run

WORKDIR /app
COPY --from=builder /app/ordersystem /app/ordersystem

EXPOSE 3000
CMD ["/app/ordersystem"]
