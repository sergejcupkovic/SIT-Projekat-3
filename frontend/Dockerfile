FROM nginx:alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY . /usr/share/nginx/html
# HTTPS only; TLS certificate/key are mounted at /etc/nginx/certs at runtime.
EXPOSE 443
CMD ["nginx", "-g", "daemon off;"]
