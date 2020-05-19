1.10.0 / 2020-05-15
========================

  * Split ci tests into multiple jobs (#304) (@payes)
  * Avoid calling AutoRmReplica on replica restarts (#300) (@payes)
  * Make the docker images configurable (@kmova)
  * Make sync http client timeout configurable (#301) (@utkarshmani1997)
  * Add command to get rebuild estimation (#297) (@utkarshmani1997)
  * Make preload operations of secondary replicas lockless (#296) (@payes)
  * Add option to flush log to file (#290) (@utkarshmani1997)
  * Remove headfile if already exists (#291) (@utkarshmani1997)
  * Run fsync on files & dir after create/remove/rename operation on files (#278) (@utkarshmani1997)

1.10.0-RC2 / 2020-05-13
========================

  * Split ci tests into multiple jobs (#304) (@payes)

1.10.0-RC1 / 2020-05-07
========================

  * Avoid calling AutoRmReplica on replica restarts (#300) (@payes)
  * Make the docker images configurable (@kmova)
  * Make sync http client timeout configurable (#301) (@utkarshmani1997)
  * Add command to get rebuild estimation (#297) (@utkarshmani1997)
  * Make preload operations of secondary replicas lockless (#296) (@payes)
  * Add option to flush log to file (#290) (@utkarshmani1997)
  * Remove headfile if already exists (#291) (@utkarshmani1997)
  * Run fsync on files & dir after create/remove/rename operation on files (#278) (@utkarshmani1997)

1.9.0-RC1 / 2020-04-07
========================

  *  fix metafile corruption at sync time ([#286](https://www.github.com/openebs/jiva#286), [@utkarshmani1997](https://github.com/utkarshmani1997))
  *  Get usedlogical size info from healthy replica and add snapshot info in volume stats ([#287](https://www.github.com/openebs/jiva#287), [@utkarshmani1997](https://github.com/utkarshmani1997))
  *  Improve logging for REST API error ([#289](https://www.github.com/openebs/jiva#289), [@utkarshmani1997](https://github.com/utkarshmani1997))
  *  Adding build for ppc64le  ([#279](https://www.github.com/openebs/jiva#279), [@pensu](https://github.com/Pensu))
  *  Update snapshot name in snapshot's metafile ([#285](https://www.github.com/openebs/jiva#285), [@utkarshmani1997](https://github.com/utkarshmani1997))

1.7.0-RC1 / 2020-02-05
========================

  *  Generate url for resize action on volume ([#266](https://www.github.com/openebs/jiva#266), [@utkarshmani1997](https://github.com/utkarshmani1997))
