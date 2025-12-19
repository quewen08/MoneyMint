# -*- coding: utf-8 -*-
from setuptools import setup, find_packages

with open('requirements.txt') as f:
    requirements = f.read().splitlines()

with open('README.md', encoding='utf-8') as f:
    long_description = f.read()

setup(
    name='moneymint-backend',
    version='0.1.0',
    packages=find_packages(),
    include_package_data=True,
    description='A personal finance management system backend based on Beancount',
    long_description=long_description,
    long_description_content_type='text/markdown',
    author='MoneyMint Team',
    author_email='contact@moneymint.example',
    url='https://github.com/moneymint/moneymint',
    install_requires=requirements,
    classifiers=[
        'Development Status :: 3 - Alpha',
        'Environment :: Web Environment',
        'Intended Audience :: End Users/Desktop',
        'License :: OSI Approved :: MIT License',
        'Programming Language :: Python :: 3',
        'Programming Language :: Python :: 3.8',
        'Programming Language :: Python :: 3.9',
        'Programming Language :: Python :: 3.10',
        'Programming Language :: Python :: 3.11',
        'Framework :: Flask',
        'Topic :: Office/Business :: Financial :: Accounting',
    ],
    entry_points={
        'console_scripts': [
            'moneymint-backend = run:app',
        ],
    },
)
