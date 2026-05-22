FROM node:20-alpine

WORKDIR /app
COPY frontend/web/package.json ./
COPY frontend/web/server.js ./
COPY frontend/web/public ./public
EXPOSE 3000
CMD ["node", "server.js"]
