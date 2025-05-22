import { Link } from '@tanstack/react-router';

function formatHandle(handle: string | null) {
  if (!handle) return '';
  const parts = handle.split('.');
  if (parts.length > 1) {
    return `${parts[0]}@${parts.slice(1).join('.')}`;
  }
  return handle;
}

interface HeaderProps {
  isAuthenticated: boolean
  handle: string | undefined
  onLogout: () => void
}

const Header = ({ isAuthenticated, handle, onLogout }: HeaderProps) => {
  return (
    <header >
      <nav>
        <ul>
          <li><Link to="/">🌱 Habitat</Link></li>
        </ul>
        {isAuthenticated ? (
          <ul >
            <li>
              {handle && formatHandle(handle)}
            </li>
            <li>
              <button onClick={onLogout}>
                Logout
              </button>
            </li>
          </ul>
        ) : (
          <ul>
            <li><Link to="/login"><button>Login</button></Link></li>
          </ul>
        )}
      </nav>
    </header>
  );
};

export default Header;
