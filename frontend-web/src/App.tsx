import { RouterProvider } from 'react-router-dom';
import { router } from './core/router';
import { ApplicationBoundary68 } from './core/recovery68/Recovery68';

function App() {
  return (
    <ApplicationBoundary68><RouterProvider router={router} /></ApplicationBoundary68>
  );
}

export default App;
