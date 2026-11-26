FROM nginx:alpine
# At start-up the script turns the template into the site config, using config.yaml
# (mounted at /etc/stashr/config.yaml, see docker-compose.yml).
COPY nginx.conf.template /etc/nginx/stashr.conf.template
COPY render-nginx-conf.sh /docker-entrypoint.d/40-stashr-config.sh
# The image only runs executable scripts, and they need Unix line endings.
RUN sed -i 's/\r$//' /docker-entrypoint.d/40-stashr-config.sh && chmod +x /docker-entrypoint.d/40-stashr-config.sh
COPY . /usr/share/nginx/html
# HTTPS only; TLS certificate/key are mounted at /etc/nginx/certs at runtime.
EXPOSE 443
CMD ["nginx", "-g", "daemon off;"]
