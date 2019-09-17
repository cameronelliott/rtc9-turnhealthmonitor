#!/bin/sh

# If command starts with an option, prepend with turnserver binary.
if [ "${1:0:1}" == '-' ]; then
  set -- /app/main "$@"
fi

exec $(eval "echo $@")
