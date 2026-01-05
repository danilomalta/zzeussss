import { useEffect, useRef } from 'react'

/**
 * Custom hook to detect clicks outside an element
 * @param {Function} handler - Callback function to execute when click outside occurs
 * @returns {React.RefObject} Ref to attach to the element
 */
export default function useClickOutside(handler) {
  const ref = useRef(null)

  useEffect(() => {
    const handleClickOutside = (event) => {
      if (ref.current && !ref.current.contains(event.target)) {
        handler(event)
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    document.addEventListener('touchstart', handleClickOutside)

    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('touchstart', handleClickOutside)
    }
  }, [handler])

  return ref
}


