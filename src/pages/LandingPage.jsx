import { motion, useScroll, useTransform } from 'framer-motion'

export default function LandingPage() {
  const { scrollYProgress } = useScroll()
  const y1 = useTransform(scrollYProgress, [0, 1], [0, -200])
  const y2 = useTransform(scrollYProgress, [0, 1], [0, -400])

  return (
    <div>
      <section className="container" style={{ padding: '80px 0 60px' }}>
        <div style={{ textAlign: 'center' }}>
          <motion.h1 style={{ margin: 0 }} initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.6 }}>
            ZZeus — Software simples, potente e leve
          </motion.h1>
          <motion.p style={{ color: 'var(--color-muted)' }} initial={{ opacity: 0 }} animate={{ opacity: 1 }} transition={{ delay: 0.2 }}>
            Clean, dynamic, professional. Built for performance on every device.
          </motion.p>
        </div>
      </section>

      <section style={{ position: 'relative', height: 600, overflow: 'hidden' }}>
        <motion.div style={{ position: 'absolute', inset: 0, y: y1, willChange: 'transform', background: 'radial-gradient(600px 300px at 50% 30%, color-mix(in srgb, var(--color-primary) 35%, transparent), transparent)' }} />
        <motion.div style={{ position: 'absolute', inset: 0, y: y2, willChange: 'transform', background: 'radial-gradient(800px 400px at 50% 70%, color-mix(in srgb, var(--color-text) 10%, transparent), transparent)' }} />
      </section>

      <section className="container" style={{ padding: '80px 0 120px', display: 'grid', gap: 24 }}>
        <motion.div initial={{ opacity: 0, y: 10 }} whileInView={{ opacity: 1, y: 0 }} viewport={{ once: true }} transition={{ duration: 0.5 }}>
          <h2>História que se revela ao rolar</h2>
          <p style={{ color: 'var(--color-muted)' }}>
            Conforme você desce, a experiência guia você pelo que nos torna a melhor solução.
          </p>
        </motion.div>
        <motion.div initial={{ opacity: 0, y: 10 }} whileInView={{ opacity: 1, y: 0 }} viewport={{ once: true }} transition={{ duration: 0.5, delay: 0.1 }}>
          <h3>Performance em foco</h3>
          <p style={{ color: 'var(--color-muted)' }}>
            Animações GPU-accelerated, assets leves e UX refinada inspirada pelos melhores.
          </p>
        </motion.div>
      </section>
    </div>
  )
}
