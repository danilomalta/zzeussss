import { motion, useScroll, useTransform } from 'framer-motion'
import React from 'react'
import AuthForm from '../components/features/AuthForm.jsx'
import useLocalization from '../hooks/useLocalization.js'

function ScrollingStory() {
  const t = useLocalization()
  const scrollRef = React.useRef(null)
  const { scrollYProgress: scrollYProgress1 } = useScroll({
    target: scrollRef,
    offset: ['start end', 'end start'],
  })
  const scale1 = useTransform(scrollYProgress1, [0.1, 0.4, 0.6, 0.8], [0.8, 1, 1, 0.8])
  const opacity1 = useTransform(scrollYProgress1, [0.1, 0.4, 0.6, 0.8], [0.5, 1, 1, 0.5])
  const y1 = useTransform(scrollYProgress1, [0.1, 0.8], ['10vh', '-10vh'])

  const { scrollYProgress: scrollYProgress2 } = useScroll({
    target: scrollRef,
    offset: ['start end', 'end start'],
  })
  const scale2 = useTransform(scrollYProgress2, [0.3, 0.5, 0.7, 0.9], [0.8, 1, 1, 0.8])
  const opacity2 = useTransform(scrollYProgress2, [0.3, 0.5, 0.7, 0.9], [0.5, 1, 1, 0.5])
  const y2 = useTransform(scrollYProgress2, [0.3, 0.9], ['10vh', '-10vh'])

  const Card = ({ text, style }) => (
    <motion.div className="story-card" style={style}>
      <div className="story-card-image-placeholder" />
      <p>{text}</p>
    </motion.div>
  )

  return (
    <section ref={scrollRef} className="scrolling-story-container">
      <motion.div initial={{ opacity: 0, y: 10 }} whileInView={{ opacity: 1, y: 0 }} viewport={{ once: true }} transition={{ duration: 0.5 }}>
        <h2>{t('story.title')}</h2>
      </motion.div>
      <div className="story-sticky-wrapper">
        <Card text={t('story.slide1')} style={{ scale: scale1, opacity: opacity1, y: y1 }} />
        <Card text={t('story.slide2')} style={{ scale: scale2, opacity: opacity2, y: y2 }} />
      </div>
      <div style={{ height: '50vh' }} />
    </section>
  )
}

export default function LandingPage() {
  const t = useLocalization()
  return (
    <div className="landing-grid-container">
      <div className="landing-left-column">
        <section className="container" style={{ padding: '80px 0 60px' }}>
          <div style={{ maxWidth: 500 }}>
            <motion.h1 style={{ margin: 0, fontSize: '2.5rem' }} initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.6 }}>
              {t('landing.title')}
            </motion.h1>
            <motion.p style={{ color: 'var(--color-muted)', fontSize: '1.1rem' }} initial={{ opacity: 0 }} animate={{ opacity: 1 }} transition={{ delay: 0.2 }}>
              {t('landing.subtitle')}
            </motion.p>
          </div>
        </section>
        <ScrollingStory />
      </div>
      <aside className="landing-right-column">
        <div className="auth-form-wrapper">
          <AuthForm onRegistered={() => {
            alert('Cadastro realizado! (WIP: Redirecionar para onboarding)')
          }} />
        </div>
      </aside>
    </div>
  )
}
