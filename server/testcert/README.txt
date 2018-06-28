The files in this directory are self signed certificates

    USE FOR TESTING PURPOSES ONLY

They can be generated using the following command:

openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 9999 -nodes -subj '/CN=localhost'

