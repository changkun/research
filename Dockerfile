# Copyright 2020 Changkun Ou. All rights reserved.

FROM alpine
COPY talks /app/talks
EXPOSE 80
CMD ["/app/talks"]