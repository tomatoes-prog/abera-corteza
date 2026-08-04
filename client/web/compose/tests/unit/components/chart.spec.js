/* eslint-disable no-unused-expressions */
import { expect } from 'chai'
import Chart from 'corteza-webapp-compose/src/components/Chart'

describe('Chart.vue', () => {
  it('optimizes category labels on vertical and horizontal bar charts', () => {
    const context = {
      chart: {
        config: {
          reports: [{ metrics: [{ type: 'bar' }] }],
        },
      },
      renderer: {
        xAxis: { type: 'value' },
        yAxis: {
          type: 'category',
          data: ['Una categoría horizontal demasiado larga'],
        },
      },
      $set: (target, key, value) => {
        target[key] = value
      },
    }

    Chart.methods.optimizeCategoricalBars.call(context)

    expect(context.renderer.yAxis.axisLabel.interval).to.equal(0)
    expect(context.renderer.yAxis.axisLabel.rotate).to.equal(0)
    expect(context.renderer.yAxis.axisLabel.formatter('Una categoría horizontal demasiado larga'))
      .to.equal('Una categoría horizo…')
  })

  it('keeps diagonal labels for category x axes', () => {
    const context = {
      chart: {
        config: {
          reports: [{ metrics: [{ type: 'bar' }] }],
        },
      },
      renderer: {
        xAxis: { type: 'category', data: ['A'] },
        yAxis: { type: 'value' },
      },
      $set: (target, key, value) => {
        target[key] = value
      },
    }

    Chart.methods.optimizeCategoricalBars.call(context)

    expect(context.renderer.xAxis.axisLabel.rotate).to.equal(-35)
  })
})
