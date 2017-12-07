FROM scratch
ADD jsonator /jsonator
EXPOSE 8080
CMD ["/jsonator"]
