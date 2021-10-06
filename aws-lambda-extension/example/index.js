const http = require("http");

exports.handler = async (event) => new Promise(async (resolve, reject) => {
  const req = http.request({ host: "localhost", port: 8080, path: "/", method: "POST" },
    (res) => {
      let buffer = "";
      res.on("data", (chunk) => (buffer += chunk));
      res.on("end", () => resolve({ statusCode: 200, body: JSON.parse(buffer) })
      );
    }
  );
  req.on("error", (e) => reject({ statusCode: 500, body: e.message }));
  req.write(JSON.stringify(event));
  req.end();
});
