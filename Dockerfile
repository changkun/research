# Copyright 2020 Changkun Ou. All rights reserved.

FROM alpine
COPY research /app/research
EXPOSE 80
CMD ["/app/research"]