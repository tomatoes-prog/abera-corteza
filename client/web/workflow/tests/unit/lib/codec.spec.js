/* eslint-disable no-unused-expressions */
import { expect } from 'chai'
import { encodeGraph } from '../../../src/lib/codec'

describe('workflow codec', () => {
  it('serializes a connected edge with the target as child', () => {
    const root = { id: '1' }
    const source = {
      id: '10',
      vertex: true,
      value: 'Source',
      parent: root,
      geometry: { x: 0, y: 0, width: 200, height: 80 },
      edges: [],
    }
    const target = {
      id: '20',
      vertex: true,
      value: 'Target',
      parent: root,
      geometry: { x: 320, y: 0, width: 200, height: 80 },
      edges: [],
    }
    const edge = {
      id: 'edge-10-20',
      value: '',
      parent: root,
      source,
      target,
      geometry: { points: [] },
      style: 'exitX=1;entryX=0;',
    }
    source.edges = [edge]
    target.edges = [edge]

    const encoded = encodeGraph(
      { cells: { root, source, target, edge } },
      {
        10: { config: { kind: 'expressions' } },
        20: { config: { kind: 'termination' } },
      },
      {},
    )

    expect(encoded.paths).to.have.length(1)
    expect(encoded.paths[0].parentID).to.equal('10')
    expect(encoded.paths[0].childID).to.equal('20')
    expect(encoded.steps[0].meta.visual).to.not.have.property('edges')
  })
})
