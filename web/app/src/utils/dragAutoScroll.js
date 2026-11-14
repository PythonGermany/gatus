// Scrolls the window while a dragged pointer is held close to the top or bottom edge of the viewport.

const EDGE_ZONE_PX = 96
const EDGE_ZONE_MAX_RATIO = 0.2
const MAX_SPEED_PX = 18

// getPointerY returns the pointer's clientY, or null to pause scrolling.
// onFrame is called on every animation frame after scrolling, e.g. to re-check what is under a stationary pointer.
export const createAutoScroller = ({ getPointerY, onFrame = () => {} }) => {
  let frame = null

  const scrollSpeed = (y) => {
    const zone = Math.min(EDGE_ZONE_PX, window.innerHeight * EDGE_ZONE_MAX_RATIO)
    if (y < zone) {
      return -MAX_SPEED_PX * Math.min(1, (zone - y) / zone)
    }
    if (y > window.innerHeight - zone) {
      return MAX_SPEED_PX * Math.min(1, (y - (window.innerHeight - zone)) / zone)
    }
    return 0
  }

  const step = () => {
    frame = requestAnimationFrame(step)
    const y = getPointerY()
    if (y !== null && y !== undefined) {
      const speed = scrollSpeed(y)
      if (speed !== 0) {
        window.scrollBy(0, Math.round(speed) || Math.sign(speed))
      }
    }
    onFrame()
  }

  return {
    start() {
      if (frame === null) frame = requestAnimationFrame(step)
    },
    stop() {
      if (frame !== null) cancelAnimationFrame(frame)
      frame = null
    }
  }
}
