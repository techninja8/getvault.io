import jwt
import datetime

SECRET_KEY = "your_secret_key"

class Authentication:
    @staticmethod
    def login(credentials):
        # Validate credentials (this is just a dummy check)
        if credentials['username'] == 'admin' and credentials['password'] == 'password':
            token = jwt.encode(
                {"user": credentials['username'], "exp": datetime.datetime.utcnow() + datetime.timedelta(hours=1)},
                SECRET_KEY,
                algorithm="HS256"
            )
            return token
        return None

    @staticmethod
    def logout(token):
        # Invalidate the token (this would be more complex in a real system)
        return True

class AccessControl:
    @staticmethod
    def check_permission(token, resource):
        try:
            decoded = jwt.decode(token, SECRET_KEY, algorithms=["HS256"])
            # Check if user has access to the resource (dummy check)
            if decoded['user'] == 'admin':
                return True
        except jwt.ExpiredSignatureError:
            return False
        except jwt.InvalidTokenError:
            return False
        return False

    @staticmethod
    def grant_access(user_id, resource):
        # Grant access to the resource (dummy implementation)
        return True