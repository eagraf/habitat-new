import { createContext, useContext } from "react";
type AuthContextType = {
  auth: any; // or your user type
  setAuth: React.Dispatch<React.SetStateAction<any>>;
};

const AuthContext = createContext<AuthContextType | null>(null);
export const useAuth = () => useContext(AuthContext);

export default AuthContext;